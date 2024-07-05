package linters

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

func init() {
	register.Plugin("example", New)
}

type MySettings struct {
	One   string    `json:"one"`
	Two   []Element `json:"two"`
	Three Element   `json:"three"`
}

type Element struct {
	Name string `json:"name"`
}

type PluginExample struct {
	settings MySettings
}

func New(settings any) (register.LinterPlugin, error) {
	// The configuration type will be map[string]any or []interface, it depends on your configuration.
	// You can use https://github.com/go-viper/mapstructure to convert map to struct.

	s, err := register.DecodeSettings[MySettings](settings)
	if err != nil {
		return nil, err
	}

	return &PluginExample{settings: s}, nil
}

func (f *PluginExample) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{
		{
			Name: "enforceMethods",
			Doc:  "Confirms that each struct implements Validator interface and has GetResourceMappings method",
			Run:  f.run,
		},
	}, nil
}

func (f *PluginExample) GetLoadMode() string {
	return register.LoadModeSyntax
}

// func run(pass *analysis.Pass) (interface{}, error) {
// 	for _, file := range pass.Files {
// 		ast.Inspect(file, func(n ast.Node) bool {
// 			if comment, ok := n.(*ast.Comment); ok {
// 				if strings.HasPrefix(comment.Text, "// TODO:") || strings.HasPrefix(comment.Text, "// TODO():") {
// 					pass.Report(analysis.Diagnostic{
// 						Pos:            comment.Pos(),
// 						End:            0,
// 						Category:       "todo",
// 						Message:        "TODO comment has no author",
// 						SuggestedFixes: nil,
// 					})
// 				}
// 			}

// 			return true
// 		})
// 	}

// 	return nil, nil
// }

type TokenInfo struct {
	pos                 token.Pos
	hasValidate         bool
	hasResourceMappings bool
}

func (f *PluginExample) run(pass *analysis.Pass) (interface{}, error) {
	structMap := make(map[string]TokenInfo)

	// Inspect struct declarations and add them to structMap
	inspectStruct := func(node ast.Node) bool {
		genDecl, ok := node.(*ast.GenDecl)
		if !ok {
			return true
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			_, ok = typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			structMap[typeSpec.Name.Name] = TokenInfo{
				pos:                 typeSpec.Pos(),
				hasValidate:         false,
				hasResourceMappings: false,
			}
		}
		return true
	}

	for _, f := range pass.Files {
		ast.Inspect(f, inspectStruct)
	}

	// Inspect function declarations to find Validate and GetResourceMappings methods
	inspectFunc := func(node ast.Node) bool {
		funcDecl, ok := node.(*ast.FuncDecl)
		if !ok {
			return true
		}

		receiverName := getReceiverName(pass.TypesInfo, funcDecl)
		if receiverName == "" {
			return true
		}

		if tokenInfo, ok := structMap[receiverName]; ok {
			if funcDecl.Name.Name == "Validate" && isValidValidateSignature(funcDecl.Type.Results.List) {
				tokenInfo.hasValidate = true
				structMap[receiverName] = tokenInfo
			} else if funcDecl.Name.Name == "GetResourceMappings" && isValidResourceMappingsSignature(funcDecl.Type.Results.List) {
				tokenInfo.hasResourceMappings = true
				structMap[receiverName] = tokenInfo
			}
		}

		return true
	}

	for _, f := range pass.Files {
		ast.Inspect(f, inspectFunc)
	}

	// Report structs without required methods
	for key, value := range structMap {
		if !value.hasValidate {
			pass.Reportf(value.pos, "struct %s does not implement method 'Validate() error'", key)
		}
		if !value.hasResourceMappings {
			pass.Reportf(value.pos, "struct %s does not implement method 'GetResourceMappings() []types.ResourceMapping'", key)
		}
	}

	return nil, nil
}

func getReceiverName(typesInfo *types.Info, funcDecl *ast.FuncDecl) string {
	if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
		return ""
	}
	recvType := typesInfo.TypeOf(funcDecl.Recv.List[0].Type)
	namedType, ok := recvType.(*types.Named)
	if !ok {
		return ""
	}
	return namedType.Obj().Name()
}

func isValidValidateSignature(results []*ast.Field) bool {
	if len(results) != 1 {
		return false
	}
	ident, ok := results[0].Type.(*ast.Ident)
	return ok && ident.Name == "error"
}

func isValidResourceMappingsSignature(results []*ast.Field) bool {
	if len(results) != 1 {
		return false
	}
	arrayType, ok := results[0].Type.(*ast.ArrayType)
	if !ok {
		return false
	}
	selectorExpr, ok := arrayType.Elt.(*ast.SelectorExpr)
	return ok && selectorExpr.Sel.Name == "ResourceMapping"
}
