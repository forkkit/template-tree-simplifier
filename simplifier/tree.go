package simplifier

import (
	"fmt"
	// "github.com/mh-cbon/print-template-tree/printer"
	"text/template/parse"
)

// Simplify browse the tree nodes
// to reduce its complexity.
func Simplify(tree *parse.Tree) {
	s := &treeSimplifier{}
	s.process(tree)
}

// treeSimplifier holds,
// a nodesDepth a stack of node supposingly it is possible to add Action before (if, range, with, action),
// the tree to modify
// vars an int to keep track of declared variable.
type treeSimplifier struct {
	nodesDepth []parse.Node
	tree       *parse.Tree
	vars       int
}

// enter pushes a node on the stack of *interesting* nodes.
func (t *treeSimplifier) enter(l parse.Node) {
	t.nodesDepth = append(t.nodesDepth, l)
}

// leave remove a node from the stack.
func (t *treeSimplifier) leave() {
	t.nodesDepth = append(t.nodesDepth[0:0], t.nodesDepth[0:len(t.nodesDepth)-1]...)
}

// current returns the current *interesting* node.
func (t *treeSimplifier) current() parse.Node {
	return t.nodesDepth[len(t.nodesDepth)-1]
}

// reset clear the node stack, usefull to browse the tree again,
// vars is not reset to keep track among iterations.
func (t *treeSimplifier) reset() {
	t.nodesDepth = make([]parse.Node, 0)
}

// process the tree until no more simplification can be done.
func (t *treeSimplifier) process(tree *parse.Tree) {
	t.tree = tree
	renameVariables(tree.Root)
	for t.browseNodes(tree.Root) {
		// printer.PrintContent(tree) // useful for debug sometimes.
		t.reset()
	}
}

// createVarName allocate a new variable name.
func (t *treeSimplifier) createVarName() string {
	name := fmt.Sprintf("$var%v", t.vars)
	t.vars++
	return name
}

// browseNodes recursively, it returns true when the tree was modified, false otherwise.
func (t *treeSimplifier) browseNodes(l interface{}) bool {
	switch node := l.(type) {

	case *parse.ListNode:
		if node != nil {
			for _, child := range node.Nodes {
				if t.browseNodes(child) {
					return true
				}
			}
		}

	case *parse.ActionNode:
		if t.simplifyActionNode(node) {
			return true
		}
		if t.variablifyActionNode(node) {
			return true
		}
		t.enter(node)
		if t.browseNodes(node.Pipe) {
			return true
		}
		t.leave()

	case *parse.PipeNode:
		ref := t.current()
		if t.simplifyPipeNode(node, ref) {
			return true
		}
		for _, child := range node.Decl {
			if t.browseNodes(child) {
				return true
			}
		}
		for _, child := range node.Cmds {
			if t.browseNodes(child) {
				return true
			}
		}

	case *parse.CommandNode:
		for _, child := range node.Args {
			if t.browseNodes(child) {
				return true
			}
		}

	case *parse.RangeNode:
		t.enter(node)
		if t.simplifyRangeNode(node) {
			return true
		}
		if t.browseNodes(node.Pipe) {
			return true
		}
		if t.browseNodes(node.List) {
			return true
		}
		if t.browseNodes(node.ElseList) {
			return true
		}
		t.leave()

	case *parse.IfNode:
		t.enter(node)
		if t.simplifyIfNode(node) {
			return true
		}
		if t.browseNodes(node.Pipe) {
			return true
		}
		if t.browseNodes(node.List) {
			return true
		}
		if t.browseNodes(node.ElseList) {
			return true
		}
		t.leave()

	case *parse.WithNode:
		t.enter(node)
		if t.simplifyWithNode(node) {
			return true
		}
		if t.browseNodes(node.Pipe) {
			return true
		}
		if t.browseNodes(node.List) {
			return true
		}
		if t.browseNodes(node.ElseList) {
			return true
		}
		t.leave()

	case *parse.TemplateNode:
		t.enter(node)
		if node.Pipe != nil {
			if t.simplifyTemplateNode(node) {
				return true
			}
			if t.browseNodes(node.Pipe) {
				return true
			}
		}
		t.leave()

	case *parse.VariableNode:
		//pass
	case *parse.IdentifierNode:
		//pass
	case *parse.StringNode:
		//pass
	case *parse.NumberNode:
		//pass
	case *parse.BoolNode:
		//pass
	case *parse.DotNode:
		//pass
	case *parse.FieldNode:
		//pass
	case *parse.TextNode:
		//pass

	default:
		err := fmt.Errorf("treeSimplifier.browseNodes: unhandled node type\n%v\n%#v", node, node)
		panic(err)
	}
	return false
}

// simplifyActionNode reduce complexity of ActionNode.
func (t *treeSimplifier) simplifyActionNode(node *parse.ActionNode) bool {
	/*
	   look for
	   {{ ("what" | up) | lower }}
	   transform into
	   {{ $some := ("what" | up) }}
	   {{ $some | lower }}
	*/
	j := len(node.Pipe.Cmds)
	lastCmd := node.Pipe.Cmds[j-1:][0]
	_, isIdent := lastCmd.Args[0].(*parse.IdentifierNode)
	// index, _ := getIndexOfCmdWithIdentifier(node.Pipe)
	// is it the last one ?
	if j > 1 && isIdent {
		// is the preceding(/first?) command a pipe/field/variable ?
		// for text/number/bool/dot node, we don t care.
		pcmd := node.Pipe.Cmds[0]
		switch pcmd.Args[0].(type) {
		case *parse.PipeNode:
			varName := t.createVarName()
			// create a new action node
			newAction := createAVariablePipeActionFromCmd(varName, nil)
			for _, c := range node.Pipe.Cmds[:j-1] {
				newAction.Pipe.Cmds = append(newAction.Pipe.Cmds, c)
			}
			// replace the cmd with the new varnode
			newCmd := createACmdNode()
			newCmd.Args = append(newCmd.Args, createAVariableNode(varName))
			node.Pipe.Cmds = []*parse.CommandNode{newCmd, lastCmd}
			// insert the new action node
			if insertActionBeforeRef(t.tree.Root, node, newAction) == false {
				err := fmt.Errorf(
					"treeSimplifier.simplifyActionNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
					newAction, newAction,
					node, node)
				panic(err)
			}
			return true
		case *parse.FieldNode:
			// its a field node, care about it,
			// only if it has arguments (a method call)
			if len(pcmd.Args) > 1 {
				varName := t.createVarName()
				// create a new action node
				newAction := createAVariablePipeActionFromCmd(varName, nil)
				for _, c := range node.Pipe.Cmds[:j-1] {
					newAction.Pipe.Cmds = append(newAction.Pipe.Cmds, c)
				}
				// replace the cmd with the new varnode
				newCmd := createACmdNode()
				newCmd.Args = append(newCmd.Args, createAVariableNode(varName))
				node.Pipe.Cmds = []*parse.CommandNode{newCmd, lastCmd}
				// insert the new action node
				if insertActionBeforeRef(t.tree.Root, node, newAction) == false {
					err := fmt.Errorf(
						"treeSimplifier.simplifyActionNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
						newAction, newAction,
						node, node)
					panic(err)
				}
				return true
			}
		case *parse.VariableNode:
			// its a variable node, care about it,
			// only if it has arguments (a method call)
			if len(pcmd.Args) > 1 {
				varName := t.createVarName()
				// create a new action node
				newAction := createAVariablePipeActionFromCmd(varName, nil)
				for _, c := range node.Pipe.Cmds[:j-1] {
					newAction.Pipe.Cmds = append(newAction.Pipe.Cmds, c)
				}
				// replace the cmd with the new varnode
				newCmd := createACmdNode()
				newCmd.Args = append(newCmd.Args, createAVariableNode(varName))
				node.Pipe.Cmds = []*parse.CommandNode{newCmd, lastCmd}
				// insert the new action node
				if insertActionBeforeRef(t.tree.Root, node, newAction) == false {
					err := fmt.Errorf(
						"treeSimplifier.simplifyActionNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
						newAction, newAction,
						node, node)
					panic(err)
				}
				return true
			}
		}
	}

	/*
	  look for
	  {{ "some" | split ("what" | up) }}
	  transform into
	  {{ $some := ("what" | up )}}
	  {{ "some" | split $some }}
	*/
	for _, cmd := range node.Pipe.Cmds {
		_, pipeToMove := getPipeFollowingIdentifier(cmd)
		if pipeToMove != nil {
			varName := t.createVarName()
			varNode := createAVariableNode(varName)
			if replacePipeWithVar(cmd, pipeToMove, varNode) == false {
				err := fmt.Errorf("treeSimplifier.simplifyActionNode: failed to replace Pipe with Var in Cmd\n%v\n%#v", cmd, cmd)
				panic(err)
			}
			newAction := createAVariablePipeAction(varName, pipeToMove)
			if insertActionBeforeRef(t.tree.Root, node, newAction) == false {
				err := fmt.Errorf(
					"treeSimplifier.simplifyActionNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
					newAction, newAction,
					node, node)
				panic(err)
			}
			return true
		}
	}
	/*
	  look for
	  {{ .Field.Node }}
	  transform into
	  {{ $some := .Field.Node}}
	  {{ $some }}
	*/
	if len(node.Pipe.Decl) == 0 && len(node.Pipe.Cmds) > 0 && len(node.Pipe.Cmds[0].Args) > 0 {
		validNode := false
		if _, ok := node.Pipe.Cmds[0].Args[0].(*parse.FieldNode); ok {
			validNode = ok
		} else if v, ok := node.Pipe.Cmds[0].Args[0].(*parse.VariableNode); ok {
			validNode = ok && len(v.Ident) > 1
		}
		if validNode {
			// transform this node into an asignment
			varName := t.createVarName()
			varNode := createAVariableNode(varName)
			node.Pipe.Decl = append(node.Pipe.Decl, varNode)
			// add a new print action node
			newAction := createActionNodeToPrintVar(varName)
			if insertActionAfterRef(t.tree.Root, node, newAction) == false {
				err := fmt.Errorf(
					"treeSimplifier.simplifyActionNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
					newAction, newAction,
					node, node)
				panic(err)
			}
			return true
		}
	}
	/*
			  look for
			  {{ split "r" .Field.Node }}
		    or
			  {{ $x := split "r" .Field.Node }}
			  transform into
			  {{ $some := .Field.Node}}
			  {{ split "r" $some }}
	*/
	if len(node.Pipe.Cmds) > 0 && len(node.Pipe.Cmds[0].Args) > 1 {
		if _, ok := node.Pipe.Cmds[0].Args[0].(*parse.IdentifierNode); ok {
			for i, arg := range node.Pipe.Cmds[0].Args {
				if field, ok := arg.(*parse.FieldNode); ok {
					// create a new assignment of the fieldNode
					varName := t.createVarName()
					newAction := createAVariableAssignmentOfFieldNode(varName, field)
					// insert the new action before this node
					if insertActionBeforeRef(t.tree.Root, node, newAction) == false {
						err := fmt.Errorf(
							"treeSimplifier.simplifyActionNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
							newAction, newAction,
							node, node)
						panic(err)
					}
					// replace the fieldNode arg with a variable node
					varNode := createAVariableNode(varName)
					node.Pipe.Cmds[0].Args[i] = varNode
					return true
				} else if varnode, ok := arg.(*parse.VariableNode); ok && len(varnode.Ident) > 1 {
					// create a new assignment of the VariableNode
					varName := t.createVarName()
					newAction := createAVariableAssignmentOfVariableNode(varName, varnode)
					// insert the new action before this node
					if insertActionBeforeRef(t.tree.Root, node, newAction) == false {
						err := fmt.Errorf(
							"treeSimplifier.simplifyActionNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
							newAction, newAction,
							node, node)
						panic(err)
					}
					// replace the fieldNode arg with a variable node
					varNode := createAVariableNode(varName)
					node.Pipe.Cmds[0].Args[i] = varNode
					return true
				}
			}
		}
	}
	return false
}

// simplifyIfNode reduce complexity of IfNode.
func (t *treeSimplifier) simplifyIfNode(node *parse.IfNode) bool {
	/* look for
	{{if .Field.Node}}
	transform to
	{{$some := .Field.Node}}{{if $some}}
	*/
	if len(node.Pipe.Cmds) > 0 && len(node.Pipe.Cmds[0].Args) == 1 {
		if field, ok := node.Pipe.Cmds[0].Args[0].(*parse.FieldNode); ok {
			varName := t.createVarName()
			varNode := createAVariableNode(varName)
			newAction := createAVariableAssignmentOfFieldNode(varName, field)
			node.Pipe.Cmds[0].Args[0] = varNode
			if insertActionBeforeRef(t.tree.Root, node, newAction) == false {
				err := fmt.Errorf(
					"treeSimplifier.simplifyIfNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
					newAction, newAction,
					node, node)
				panic(err)
			}
			return true
		}
	}
	return false
}

// simplifyTemplateNode reduce complexity of TemplateNode.
func (t *treeSimplifier) simplifyTemplateNode(node *parse.TemplateNode) bool {
	/* look for
	{{template "rr" .Field.Node}}
	transform to
	{{$some := .Field.Node}}{{template "rr" $some}}
	*/
	if len(node.Pipe.Cmds) > 0 && len(node.Pipe.Cmds[0].Args) == 1 {
		if field, ok := node.Pipe.Cmds[0].Args[0].(*parse.FieldNode); ok {
			varName := t.createVarName()
			varNode := createAVariableNode(varName)
			newAction := createAVariableAssignmentOfFieldNode(varName, field)
			node.Pipe.Cmds[0].Args[0] = varNode
			if insertActionBeforeRef(t.tree.Root, node, newAction) == false {
				err := fmt.Errorf(
					"treeSimplifier.simplifyIfNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
					newAction, newAction,
					node, node)
				panic(err)
			}
			return true
		}
	}
	return false
}

// simplifyWithNode reduce complexity of WithNode.
func (t *treeSimplifier) simplifyWithNode(node *parse.WithNode) bool {
	if len(node.Pipe.Cmds) > 0 && len(node.Pipe.Cmds[0].Args) == 1 {
		/* look for
		{{with $y := .S.S}}
		transform to
		{{$some := .Field.Node}}{{with $y := $some}}
		*/
		if field, ok := node.Pipe.Cmds[0].Args[0].(*parse.FieldNode); ok {
			varName := t.createVarName()
			varNode := createAVariableNode(varName)
			newAction := createAVariableAssignmentOfFieldNode(varName, field)
			node.Pipe.Cmds[0].Args[0] = varNode
			if insertActionBeforeRef(t.tree.Root, node, newAction) == false {
				err := fmt.Errorf(
					"treeSimplifier.simplifyWithNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
					newAction, newAction,
					node, node)
				panic(err)
			}
			return true

			/* look for
			{{with .}}
			transform to
			{{$some := .}}{{with $y := $some}}
			*/
		} else if dot, ok := node.Pipe.Cmds[0].Args[0].(*parse.DotNode); ok {
			varName := t.createVarName()
			varNode := createAVariableNode(varName)
			newAction := createAVariableAssignmentOfDotNode(varName, dot)
			node.Pipe.Cmds[0].Args[0] = varNode
			if insertActionBeforeRef(t.tree.Root, node, newAction) == false {
				err := fmt.Errorf(
					"treeSimplifier.simplifyWithNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
					newAction, newAction,
					node, node)
				panic(err)
			}
			return true

			/* look for
			{{with $x.Field.Node}}
			transform to
			{{$some := $x.Field.Node}}{{with $some}}
			*/
		} else if variable, ok := node.Pipe.Cmds[0].Args[0].(*parse.VariableNode); ok && len(variable.Ident) > 1 {
			varName := t.createVarName()
			varNode := createAVariableNode(varName)
			newAction := createAVariableAssignmentOfVariableNode(varName, variable)
			node.Pipe.Cmds[0].Args[0] = varNode
			if insertActionBeforeRef(t.tree.Root, node, newAction) == false {
				err := fmt.Errorf(
					"treeSimplifier.simplifyWithNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
					newAction, newAction,
					node, node)
				panic(err)
			}
			return true
		}
	}
	return false
}

// simplifyRangeNode reduce complexity of RangeNode.
func (t *treeSimplifier) simplifyRangeNode(node *parse.RangeNode) bool {
	if len(node.Pipe.Cmds) > 0 && len(node.Pipe.Cmds[0].Args) == 1 {
		/* look for
		{{range .S.S}}
		transform to
		{{$some := .Field.Node}}{{range $some}}
		*/
		if field, ok := node.Pipe.Cmds[0].Args[0].(*parse.FieldNode); ok {
			varName := t.createVarName()
			varNode := createAVariableNode(varName)
			newAction := createAVariableAssignmentOfFieldNode(varName, field)
			node.Pipe.Cmds[0].Args[0] = varNode
			if insertActionBeforeRef(t.tree.Root, node, newAction) == false {
				err := fmt.Errorf(
					"treeSimplifier.simplifyRangeNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
					newAction, newAction,
					node, node)
				panic(err)
			}
			return true

			/* look for
			{{range .}}
			transform to
			{{$some := .}}{{range $some}}
			*/
		} else if dot, ok := node.Pipe.Cmds[0].Args[0].(*parse.DotNode); ok {
			varName := t.createVarName()
			varNode := createAVariableNode(varName)
			newAction := createAVariableAssignmentOfDotNode(varName, dot)
			node.Pipe.Cmds[0].Args[0] = varNode
			if insertActionBeforeRef(t.tree.Root, node, newAction) == false {
				err := fmt.Errorf(
					"treeSimplifier.simplifyRangeNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
					newAction, newAction,
					node, node)
				panic(err)
			}
			return true

			/* look for
			{{range $x.Field.Node}}
			transform to
			{{$some := $x.Field.Node}}{{range $some}}
			*/
		} else if variable, ok := node.Pipe.Cmds[0].Args[0].(*parse.VariableNode); ok && len(variable.Ident) > 1 {
			varName := t.createVarName()
			varNode := createAVariableNode(varName)
			newAction := createAVariableAssignmentOfVariableNode(varName, variable)
			node.Pipe.Cmds[0].Args[0] = varNode
			if insertActionBeforeRef(t.tree.Root, node, newAction) == false {
				err := fmt.Errorf(
					"treeSimplifier.simplifyRangeNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
					newAction, newAction,
					node, node)
				panic(err)
			}
			return true
		}
	}
	return false
}

// variablifyActionNode test action node which are complex
// structure, split them into
// a variable assignment,
// a variable print
func (t *treeSimplifier) variablifyActionNode(node *parse.ActionNode) bool {
	/*
	   look for
	   {{ lower "rr" }}
	   transform into
	   {{ $some := lower "rr" }}
	   {{ $some }}
	*/
	if len(node.Pipe.Decl) == 0 && len(node.Pipe.Cmds) > 0 {
		if _, ok := node.Pipe.Cmds[0].Args[0].(*parse.IdentifierNode); ok {
			varname := t.createVarName()
			//transform the print into a variable assignment
			node.Pipe.Decl = append(node.Pipe.Decl, &parse.VariableNode{
				Ident: []string{varname},
			})
			// add a print of the variable
			newAction := createActionNodeToPrintVar(varname)
			if insertActionAfterRef(t.tree.Root, node, newAction) == false {
				err := fmt.Errorf(
					"treeSimplifier.variablifyActionNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
					newAction, newAction,
					node, node)
				panic(err)
			}
			return true
		}
	}

	return false
}

// simplifyPipeNode reduce complexity of PipeNode.
func (t *treeSimplifier) simplifyPipeNode(node *parse.PipeNode, ref parse.Node) bool {
	/*
	  look for
	  {{"some" | split "what"}}
	  transform into
	  {{split "what" "some"}}
	*/
	if rearrangeCmdsWithIdentifierPrecededByCmdWithVariableNode(node) {
		return true
	}

	/*
	  look for
	  {{up "what" | lower}}
	  transform into
	  {{$some := up "what"}}
	  {{$some | lower}}
	*/
	firstCmd, secCmd := getCmdIdentifierFollowedByCmdIdentifier(node)
	if firstCmd != nil && secCmd != nil {
		firstCmdIndex := getCmdIndex(firstCmd, node)
		if firstCmdIndex > -1 {
			varName := t.createVarName()
			varNode := createAVariableNode(varName)
			if replaceCmdWithVar(node, firstCmd, varNode) == false {
				err := fmt.Errorf("treeSimplifier.simplifyPipeNode: failed to replace Pipe with Var in Cmd\n%v\n%#v", firstCmd, firstCmd)
				panic(err)
			}
			newAction := createAVariablePipeActionFromCmd(varName, firstCmd)
			if insertActionBeforeRef(t.tree.Root, ref, newAction) == false {
				err := fmt.Errorf(
					"treeSimplifier.simplifyPipeNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
					newAction, newAction,
					node, node)
				panic(err)
			}
			return true
		}
	}

	// following transform can be executed only on
	// ref node like if/else/range/with
	isValidRef := false
	switch ref.(type) {
	case *parse.IfNode:
		isValidRef = true
	case *parse.RangeNode:
		isValidRef = true
	case *parse.WithNode:
		isValidRef = true
	case *parse.TemplateNode:
		isValidRef = true
	}
	if isValidRef {
		/*
		  look for
		  {{if not true}}
		  transform into
		  {{$some := not true}}
		  {{if $some}}
		*/
		if len(node.Cmds) > 0 {
			cmd := node.Cmds[0]
			if len(cmd.Args) > 0 {
				if _, ok := cmd.Args[0].(*parse.IdentifierNode); ok {
					varName := t.createVarName()
					varNode := createAVariableNode(varName)
					newAction := createAVariablePipeAction(varName, node)
					newCmd := &parse.CommandNode{}
					newCmd.NodeType = parse.NodeCommand
					newCmd.Args = append(newCmd.Args, varNode)
					node.Cmds = append(node.Cmds[:0], newCmd)
					if insertActionBeforeRef(t.tree.Root, ref, newAction) == false {
						err := fmt.Errorf(
							"treeSimplifier.simplifyPipeNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
							newAction, newAction,
							ref, ref)
						panic(err)
					}
					return true
				}
			}
		}

		/*
		  look for
		  {{if eq (up "what" | lower) "what"}}
		  transform into
		  {{$some := eq (up "what" | lower) "what"}}
		  {{if $some}}
		*/
		if len(node.Cmds) > 0 {
			cmd := node.Cmds[0]
			_, pipeToMove := getPipeFollowingIdentifier(cmd)
			if pipeToMove != nil {
				varName := t.createVarName()
				varNode := createAVariableNode(varName)
				if replacePipeWithVar(cmd, pipeToMove, varNode) == false {
					err := fmt.Errorf("treeSimplifier.simplifyPipeNode: failed to replace Pipe with Var in Cmd\n%v\n%#v", cmd, cmd)
					panic(err)
				}
				newAction := createAVariablePipeAction(varName, pipeToMove)
				if insertActionBeforeRef(t.tree.Root, ref, newAction) == false {
					err := fmt.Errorf(
						"treeSimplifier.simplifyPipeNode: failed to insert the new Action node\n%v\n%#v\nreference node was\n%v\n%#v",
						newAction, newAction,
						ref, ref)
					panic(err)
				}
				return true
			}
		}
	}

	/*
			  look for
			  {{(up "what")}}
			  transform into
			  {{up "what"}}
		    (ie: removes parenthesis)
	*/
	if len(node.Cmds) == 1 && len(node.Cmds[0].Args) == 1 {
		if p, ok := node.Cmds[0].Args[0].(*parse.PipeNode); ok {
			node.Cmds = p.Cmds
			return true
		}
	}

	return false
}

// insertActionBeforeRef browses given node list until it can find ref node,
// it then insert newAction before the ref node.
// It returns false if it failed to insert the new node.
func insertActionBeforeRef(list *parse.ListNode, ref parse.Node, newAction *parse.ActionNode) bool {
	for i := 0; i < len(list.Nodes); i++ {
		if list.Nodes[i] == ref {
			list.Nodes = append(list.Nodes, nil)
			copy(list.Nodes[i+1:], list.Nodes[i:])
			list.Nodes[i] = newAction
			return true
		}
		switch node := list.Nodes[i].(type) {
		case *parse.IfNode:
			if node.List != nil && insertActionBeforeRef(node.List, ref, newAction) {
				return true
			}
			if node.ElseList != nil && insertActionBeforeRef(node.ElseList, ref, newAction) {
				return true
			}
		case *parse.RangeNode:
			if node.List != nil && insertActionBeforeRef(node.List, ref, newAction) {
				return true
			}
			if node.ElseList != nil && insertActionBeforeRef(node.ElseList, ref, newAction) {
				return true
			}
		case *parse.WithNode:
			if node.List != nil && insertActionBeforeRef(node.List, ref, newAction) {
				return true
			}
			if node.ElseList != nil && insertActionBeforeRef(node.ElseList, ref, newAction) {
				return true
			}
		}
	}
	return false
}

// insertActionAfterRef browses given node list until it can find ref node,
// it then insert newAction after the ref node.
// It returns false if it failed to insert the new node.
func insertActionAfterRef(list *parse.ListNode, ref parse.Node, newAction *parse.ActionNode) bool {
	for i := 0; i < len(list.Nodes); i++ {
		if list.Nodes[i] == ref {
			list.Nodes = append(list.Nodes, nil)
			copy(list.Nodes[i+2:], list.Nodes[i+1:])
			list.Nodes[i+1] = newAction
			return true
		}
		switch node := list.Nodes[i].(type) {
		case *parse.IfNode:
			if node.List != nil && insertActionAfterRef(node.List, ref, newAction) {
				return true
			}
			if node.ElseList != nil && insertActionAfterRef(node.ElseList, ref, newAction) {
				return true
			}
		case *parse.RangeNode:
			if node.List != nil && insertActionAfterRef(node.List, ref, newAction) {
				return true
			}
			if node.ElseList != nil && insertActionAfterRef(node.ElseList, ref, newAction) {
				return true
			}
		case *parse.WithNode:
			if node.List != nil && insertActionAfterRef(node.List, ref, newAction) {
				return true
			}
			if node.ElseList != nil && insertActionAfterRef(node.ElseList, ref, newAction) {
				return true
			}
		}
	}
	return false
}

// createAVariablePipeAction creates a new ActionNode as an assignment
// of a PipeNode to a new variable node.
// example:
// {{ ("what" | up) | lower }}
// the pipe to modify is: ("what" | up)
// this func will create: {{$name := ("what" | up)}}
func createAVariablePipeAction(name string, pipe *parse.PipeNode) *parse.ActionNode {
	varNode := &parse.VariableNode{
		NodeType: parse.NodeVariable,
		Ident:    []string{name},
	}
	actionPipe := &parse.PipeNode{}
	actionPipe.Decl = append(actionPipe.Decl, varNode)
	actionPipe.Cmds = append(actionPipe.Cmds, pipe.Cmds...)
	node := &parse.ActionNode{
		NodeType: parse.NodeAction,
		Pipe:     actionPipe,
	}
	return node
}

// createAVariablePipeActionFromCmd creates a new ActionNode as an assignment
// of a CommandNode to a new variable node.
// example:
// {{ up "what" | lower }}
// the command to modify is: up "what"
// this func will create: {{$name := up "what" | lower}}
func createAVariablePipeActionFromCmd(name string, cmd *parse.CommandNode) *parse.ActionNode {
	varNode := &parse.VariableNode{
		NodeType: parse.NodeVariable,
		Ident:    []string{name},
	}
	actionPipe := &parse.PipeNode{}
	actionPipe.Decl = append(actionPipe.Decl, varNode)
	if cmd != nil {
		actionPipe.Cmds = append(actionPipe.Cmds, cmd)
	}
	node := &parse.ActionNode{
		NodeType: parse.NodeAction,
		Pipe:     actionPipe,
	}
	return node
}

// createAVariableAssignmentOfSomeNode creates a new ActionNode as an assignment
// of a the given node.
// example:
// {{ .Field.Node }}
// this func will create: {{$name := .Field.Node }}
func createAVariableAssignmentOfSomeNode(name string, node parse.Node) *parse.ActionNode {
	varNode := &parse.VariableNode{
		NodeType: parse.NodeVariable,
		Ident:    []string{name},
	}
	actionPipe := &parse.PipeNode{}
	actionPipe.Decl = append(actionPipe.Decl, varNode)
	cmdNode := createACmdNode()
	cmdNode.Args = []parse.Node{node}
	actionPipe.Cmds = append(actionPipe.Cmds, cmdNode)
	return &parse.ActionNode{
		NodeType: parse.NodeAction,
		Pipe:     actionPipe,
	}
}

// createAVariableAssignmentOfFieldNode creates a new ActionNode as an assignment
// of a FieldNode.
// example:
// {{ .Field.Node }}
// this func will create: {{$name := .Field.Node }}
func createAVariableAssignmentOfFieldNode(name string, f *parse.FieldNode) *parse.ActionNode {
	return createAVariableAssignmentOfSomeNode(name, f)
}

// createAVariableAssignmentOfDotNode creates a new ActionNode as an assignment
// of a DotNode.
// example:
// {{ . }}
// this func will create: {{$name := . }}
func createAVariableAssignmentOfDotNode(name string, f *parse.DotNode) *parse.ActionNode {
	return createAVariableAssignmentOfSomeNode(name, f)
}

// createAVariableAssignmentOfVariableNode creates a new ActionNode as an assignment
// of a VariableNode.
// example:
// {{ $x }}
// this func will create: {{$name := $x }}
func createAVariableAssignmentOfVariableNode(name string, f *parse.VariableNode) *parse.ActionNode {
	return createAVariableAssignmentOfSomeNode(name, f)
}

// createActionNodeToPrintVar creates a new ActionNode to print a var
// {{$name}}
func createActionNodeToPrintVar(varname string) *parse.ActionNode {
	newAction := &parse.ActionNode{}
	newAction.NodeType = parse.NodeAction
	newAction.Pipe = &parse.PipeNode{}
	newAction.Pipe.NodeType = parse.NodePipe
	newAction.Pipe.Decl = make([]*parse.VariableNode, 0)
	newAction.Pipe.Cmds = make([]*parse.CommandNode, 0)
	cmd := createACmdNode()
	cmd.Args = append(cmd.Args, &parse.VariableNode{
		NodeType: parse.NodeVariable,
		Ident:    []string{varname},
	})
	newAction.Pipe.Cmds = append(newAction.Pipe.Cmds, cmd)
	return newAction
}

// createAVariableNode creates a VariableNode with given name.
func createAVariableNode(name string) *parse.VariableNode {
	varNode := &parse.VariableNode{
		NodeType: parse.NodeVariable,
		Ident:    []string{name},
	}
	return varNode
}

// createACmdNode creates an empty CommandNode.
func createACmdNode() *parse.CommandNode {
	cmd := &parse.CommandNode{}
	cmd.NodeType = parse.NodeCommand
	return cmd
}

// replaceCmdWithVar replaces given searched command,
// with the provided varnode within pipe.Cmds.
// It creates a new command to embed the varnode before it is inserted.
// it returns false it search node was not found.
func replaceCmdWithVar(pipe *parse.PipeNode, search *parse.CommandNode, varnode *parse.VariableNode) bool {
	for i := 0; i < len(pipe.Cmds); i++ {
		if pipe.Cmds[i] == search {
			newCmd := &parse.CommandNode{
				NodeType: parse.NodeCommand,
				Args:     make([]parse.Node, 0),
			}
			newCmd.Args = append(newCmd.Args, varnode)
			pipe.Cmds[i] = newCmd
			return true
		}
	}
	return false
}

// replacePipeWithVar replaces old Pipe contained in cmd.Args by the newnode Variable.
// It returns false if the replacement was not done.
func replacePipeWithVar(cmd *parse.CommandNode, old *parse.PipeNode, newnode *parse.VariableNode) bool {
	for i := 0; i < len(cmd.Args); i++ {
		if cmd.Args[i] == old {
			cmd.Args[i] = newnode
			return true
		}
	}
	return false
}

// getPipeFollowingIdentifier identifies a PipeNode following an IdentifierNode into given CommandNode.
// example:
// {{ "some" | split ("what" | up) }}
// pipe is: ("what" | up)
// identifier is: split
func getPipeFollowingIdentifier(node *parse.CommandNode) (*parse.IdentifierNode, *parse.PipeNode) {
	if len(node.Args) > 1 {
		if identifier, ok := node.Args[0].(*parse.IdentifierNode); ok {
			i, pipe := getIndexOfPipe(node.Args)
			if i > 0 {
				return identifier, pipe
			}
		}
	}
	return nil, nil
}

// getIndexOfPipe finds index of the first PipeNode encountered into given node list.
func getIndexOfPipe(nodes []parse.Node) (int, *parse.PipeNode) {
	for i, node := range nodes {
		if pipe, ok := node.(*parse.PipeNode); ok {
			return i, pipe
		}
	}
	return -1, nil
}

// rearrangeCmdsWithIdentifierPrecededByCmdWithVariableNode is a long name for a simple operation.
// When an IdentifierNode is preceded of a VariableNode,
// it moves the VariableNode at the end of the argument list of the IdentifierNode.
// example: {{"some" | split "what"}}
// becomes: {{split "what" "some"}}
func rearrangeCmdsWithIdentifierPrecededByCmdWithVariableNode(node *parse.PipeNode) bool {
	identifierIndex, cmd := getIndexOfCmdWithIdentifier(node)
	//-
	variableIndex, variable := getIndexOfCmdWithVariable(node)
	if variableIndex > -1 && identifierIndex > variableIndex {
		cmd.Args = append(cmd.Args, variable)
		rmCmdAtIndex(node, variableIndex)
		return true
	}
	//-
	stringIndex, str := getIndexOfCmdWithString(node)
	if stringIndex > -1 && identifierIndex > stringIndex {
		cmd.Args = append(cmd.Args, str)
		rmCmdAtIndex(node, stringIndex)
		return true
	}
	//-
	dotIndex, dot := getIndexOfCmdWithDot(node)
	if dotIndex > -1 && identifierIndex > dotIndex {
		cmd.Args = append(cmd.Args, dot)
		rmCmdAtIndex(node, dotIndex)
		return true
	}
	//-
	fieldIndex, field := getIndexOfCmdWithFieldNode(node)
	if fieldIndex > -1 && identifierIndex > fieldIndex {
		cmd.Args = append(cmd.Args, field)
		rmCmdAtIndex(node, fieldIndex)
		return true
	}
	//-
	numberIndex, number := getIndexOfCmdWithNumberNode(node)
	if numberIndex > -1 && identifierIndex > numberIndex {
		cmd.Args = append(cmd.Args, number)
		rmCmdAtIndex(node, numberIndex)
		return true
	}
	return false
}

// rmCmdAtIndex removes a command of given PipeNode at the provided index.
func rmCmdAtIndex(node *parse.PipeNode, cmdAtIndex int) {
	if len(node.Cmds) > cmdAtIndex+1 {
		node.Cmds = append(node.Cmds[0:cmdAtIndex], node.Cmds[cmdAtIndex+1:]...)
	} else {
		node.Cmds = node.Cmds[0:cmdAtIndex]
	}
}

// getIndexOfCmdWithIdentifier searches into pipe.Cmds for an IdentifierNode.
func getIndexOfCmdWithIdentifier(node *parse.PipeNode) (int, *parse.CommandNode) {
	for i, cmd := range node.Cmds {
		if len(cmd.Args) > 0 {
			if _, ok := cmd.Args[0].(*parse.IdentifierNode); ok {
				return i, cmd
			}
		}
	}
	return -1, nil
}

// getIndexOfCmdWithNumberNode searches into pipe.Cmds for a NumberNode.
func getIndexOfCmdWithNumberNode(node *parse.PipeNode) (int, *parse.NumberNode) {
	for i, cmd := range node.Cmds {
		if len(cmd.Args) == 1 { // not sure about that, worth to be restrictive.
			if s, ok := cmd.Args[0].(*parse.NumberNode); ok {
				return i, s
			}
		}
	}
	return -1, nil
}

// getIndexOfCmdWithFieldNode searches into pipe.Cmds for a FieldNode.
func getIndexOfCmdWithFieldNode(node *parse.PipeNode) (int, *parse.FieldNode) {
	for i, cmd := range node.Cmds {
		if len(cmd.Args) == 1 { // not sure about that, worth to be restrictive.
			if s, ok := cmd.Args[0].(*parse.FieldNode); ok {
				return i, s
			}
		}
	}
	return -1, nil
}

// getIndexOfCmdWithDot searches into pipe.Cmds for a DotNode.
func getIndexOfCmdWithDot(node *parse.PipeNode) (int, *parse.DotNode) {
	for i, cmd := range node.Cmds {
		if len(cmd.Args) == 1 { // not sure about that, worth to be restrictive.
			if s, ok := cmd.Args[0].(*parse.DotNode); ok {
				return i, s
			}
		}
	}
	return -1, nil
}

// getIndexOfCmdWithVariable searches into pipe.Cmds for a VariableNode.
func getIndexOfCmdWithVariable(node *parse.PipeNode) (int, *parse.VariableNode) {
	for i, cmd := range node.Cmds {
		if len(cmd.Args) == 1 { // not sure about that, worth to be restrictive.
			if s, ok := cmd.Args[0].(*parse.VariableNode); ok {
				return i, s
			}
		}
	}
	return -1, nil
}

// getIndexOfCmdWithString searches into pipe.Cmds for a StringNode.
func getIndexOfCmdWithString(node *parse.PipeNode) (int, *parse.StringNode) {
	for i, cmd := range node.Cmds {
		if len(cmd.Args) == 1 { // not sure about that, worth to be restrictive.
			if s, ok := cmd.Args[0].(*parse.StringNode); ok {
				return i, s
			}
		}
	}
	return -1, nil
}

// getCmdIndex finds index of search CommandNode within given PipeNode.
func getCmdIndex(search *parse.CommandNode, into *parse.PipeNode) int {
	for i, cmd := range into.Cmds {
		if search == cmd {
			return i
		}
	}
	return -1
}

// getCmdIdentifierFollowedByCmdIdentifier searches pipe.Cmds for an identifier followed by
// a sub command beginning with an identifier.
// example:
// {{up "what" | lower}}
// left command: up "what"
// right command: lower
func getCmdIdentifierFollowedByCmdIdentifier(node *parse.PipeNode) (*parse.CommandNode, *parse.CommandNode) {
	for i, cmd := range node.Cmds {
		if _, ok := cmd.Args[0].(*parse.IdentifierNode); ok {
			if len(node.Cmds) > i+1 {
				if _, ok := node.Cmds[i+1].Args[0].(*parse.IdentifierNode); ok {
					return cmd, node.Cmds[i+1]
				}
			}
		}
	}
	return nil, nil
}
