package simplifier

import (
	"fmt"
	"text/template/parse"
)

// renameVariables browses all tree nodes,
// renames all encountered VariableNode Ident,
// adding a prefix "tpl_".
func renameVariables(l interface{}) {
	switch node := l.(type) {

	case *parse.ListNode:
		if node != nil {
			for _, child := range node.Nodes {
				renameVariables(child)
			}
		}

	case *parse.VariableNode:
		if node.Ident[0] != "$" {
			node.Ident[0] = "$tpl_" + node.Ident[0][1:] // get ride of $ sign
		}

	case *parse.ActionNode:
		renameVariables(node.Pipe)

	case *parse.PipeNode:
		for _, child := range node.Decl {
			renameVariables(child)
		}
		for _, child := range node.Cmds {
			renameVariables(child)
		}

	case *parse.CommandNode:
		for _, child := range node.Args {
			renameVariables(child)
		}

	case *parse.RangeNode:
		renameVariables(node.Pipe)
		renameVariables(node.List)
		renameVariables(node.ElseList)

	case *parse.IfNode:
		renameVariables(node.Pipe)
		renameVariables(node.List)
		renameVariables(node.ElseList)

	case *parse.WithNode:
		renameVariables(node.BranchNode.Pipe)
		renameVariables(node.BranchNode.List)
		renameVariables(node.BranchNode.ElseList)

	case *parse.StringNode:
		// pass
	case *parse.NumberNode:
		// pass
	case *parse.BoolNode:
		// pass
	case *parse.IdentifierNode:
		// pass
	case *parse.DotNode:
		// pass
	case *parse.FieldNode:
		// pass
	case *parse.TextNode:
		// pass

	default:
		fmt.Printf("%#v\n", node)
		fmt.Printf("!!! Unhandled %T\n", node)
		panic("unhandled")
	}
}
