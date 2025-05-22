package generator

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/isheperserga/vfio2verilog/internal/parser"
)

type tmplData struct {
	Decls   string
	Inits   string
	Cases   string
	ModName string
}

const modTmpl = `module {{.ModName}}(
    input               rst,
    input               clk,
    // incoming BAR writes:
    input [31:0]        wr_addr,
    input [3:0]         wr_be,
    input [31:0]        wr_data,
    input               wr_valid,
    // incoming BAR reads:
    input  [87:0]       rd_req_ctx,
    input  [31:0]       rd_req_addr,
    input               rd_req_valid,
    input  [31:0]       base_address_register,
    // outgoing BAR read replies:
    output reg [87:0]   rd_rsp_ctx,
    output reg [31:0]   rd_rsp_data,
    output reg          rd_rsp_valid
);
    
    bit [87:0]      drd_req_ctx;
    bit [31:0]      drd_req_addr;
    bit             drd_req_valid;

    bit [31:0]      dwr_addr;
    bit [31:0]      dwr_data;
    bit             dwr_valid;
    
{{ .Decls }}

    always @ (posedge clk) begin
        if (rst) begin
            rd_rsp_valid <= 1'b0;
            
{{ .Inits }}

        end else begin
            drd_req_ctx     <= rd_req_ctx;
            drd_req_valid   <= rd_req_valid;
            dwr_valid       <= wr_valid;
            drd_req_addr    <= rd_req_addr;
            rd_rsp_ctx      <= drd_req_ctx;
            rd_rsp_valid    <= drd_req_valid;
            dwr_addr        <= wr_addr;
            dwr_data        <= wr_data;

            if (drd_req_valid) begin
                case (({drd_req_addr[31:24], drd_req_addr[23:16], drd_req_addr[15:08], drd_req_addr[07:00]} - (base_address_register & ~32'h4)) & 32'hFFFF)
{{ .Cases }}
                    default: rd_rsp_data <= 32'h00000000;
                endcase
			end else if (dwr_valid) begin
				// do reads if you wish
            end else begin
				rd_rsp_data <= 32'h00000000;
			end
        end
    end
endmodule
`

func GenerateVerilogFromOps(ops []parser.LogOp, modName string) (string, error) {
	var decl, init strings.Builder
	var cases strings.Builder

	wordSeqs := map[uint32][]uint32{}
	wordComments := map[uint32][]string{}

	for _, op := range ops {
		base := op.Addr &^ 3
		var outVal uint32
		if op.Size == 4 {
			outVal = op.Val
		} else if op.Size == 1 {
			off := op.Addr & 3
			outVal = uint32(op.Val) << (8 * off)
		}
		wordSeqs[base] = append(wordSeqs[base], outVal)
		sizeStr := fmt.Sprintf("%db", op.Size)
		initAddr := fmt.Sprintf("0x%04x", op.Addr)
		initVal := fmt.Sprintf("0x%x", op.Val)
		comment := fmt.Sprintf("// bar%d %s read from %s = %s", op.Region, sizeStr, initAddr, initVal)
		wordComments[base] = append(wordComments[base], comment)
	}

	addrs := make([]uint32, 0, len(wordSeqs))
	for addr := range wordSeqs {
		addrs = append(addrs, addr)
	}
	sort.Slice(addrs, func(i, j int) bool { return addrs[i] < addrs[j] })

	for _, addr := range addrs {
		vals := wordSeqs[addr]
		comments := wordComments[addr]
		if len(vals) == 0 {
			continue
		}
		addrHex := fmt.Sprintf("%04x", addr)
		bits := bitsNeeded(len(vals))
		decl.WriteString(fmt.Sprintf("    bit [%d:0] R_C_%s;\n", bits-1, addrHex))
		decl.WriteString(fmt.Sprintf("    bit [31:0] R_%s [0:%d];\n\n", addrHex, len(vals)-1))
		init.WriteString(fmt.Sprintf("            R_C_%s <= '0;\n", addrHex))
		for i, val := range vals {
			cmt := ""
			if i < len(comments) {
				cmt = " " + comments[i]
			}
			init.WriteString(fmt.Sprintf("            R_%s[%d] <= 32'h%08x;%s\n", addrHex, i, val, cmt))
		}
		init.WriteString("\n")
		cases.WriteString(fmt.Sprintf("                    32'h%s: begin\n", addrHex))
		cases.WriteString(fmt.Sprintf("                        rd_rsp_data <= R_%s[R_C_%s];\n", addrHex, addrHex))
		cases.WriteString(fmt.Sprintf("                        R_C_%s <= (R_C_%s == %d) ? '0 : R_C_%s + 1;\n",
			addrHex, addrHex, len(vals)-1, addrHex))
		cases.WriteString("                    end\n")
	}

	tmplStuff := tmplData{
		Decls:   decl.String(),
		Inits:   init.String(),
		Cases:   cases.String(),
		ModName: modName,
	}

	tmpl, err := template.New("verilog").Parse(modTmpl)
	if err != nil {
		return "", fmt.Errorf("tmpl err: %w", err)
	}

	var result bytes.Buffer
	if err := tmpl.Execute(&result, tmplStuff); err != nil {
		return "", fmt.Errorf("tmpl run err: %w", err)
	}

	return result.String(), nil
}

func bitsNeeded(count int) int {
	bits := 1
	for (1 << bits) < count {
		bits++
	}
	return bits
}
