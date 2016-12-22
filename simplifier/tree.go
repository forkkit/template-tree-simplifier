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
			// t.enter(node)
			for _, child := range node.Nodes {
				if t.browseNodes(child) {
					return true
				}
			}
			// t.leave()
		}

	case *parse.ActionNode:
		if t.simplifyActionNode(node) {
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
		// t.enter(node)
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
		// t.leave()

	case *parse.CommandNode:
		// t.enter(node)
		for _, child := range node.Args {
			if t.browseNodes(child) {
				return true
			}
		}
		// t.leave()

	case *parse.RangeNode:
		t.enter(node)
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

		// default:
		// 	fmt.Printf("%#v\n", node)
		// 	fmt.Printf("!!! Unhandled %T\n", node)
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
	cmd, pipeToMove := getPipePrecedingIdentifier(node.Pipe)
	if pipeToMove != nil {
		varName := t.createVarName()
		varNode := createAVariableNode(varName)
		if replacePipeWithVar(cmd, pipeToMove, varNode) == false {
			panic("pipe not found in cmd")
		}
		newAction := createAVariablePipeAction(varName, pipeToMove)
		if t.insertActionBeforeRef(t.tree.Root, node, newAction) == false {
			panic("reference node not found")
		}
		return true
	}
	// if len(node.Pipe.Decl) == 0 {
	/*
	  look for
	  {{ "some" | split ("what" | up) }}
	  transform into
	  {{ $some := ("what" | up )}}
	  {{ "some" | split $some }}
	*/
	for _, cmd := range node.Pipe.Cmds {
		_, pipeToMove = getPipeFollowingIdentifier(cmd)
		if pipeToMove != nil {
			varName := t.createVarName()
			varNode := createAVariableNode(varName)
			if replacePipeWithVar(cmd, pipeToMove, varNode) == false {
				panic("pipe not found in cmd")
			}
			newAction := createAVariablePipeAction(varName, pipeToMove)
			if t.insertActionBeforeRef(t.tree.Root, node, newAction) == false {
				panic("reference node not found")
			}
			return true
		}
	}
	// }
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
				panic("cmd not found in pipe")
			}
			newAction := createAVariablePipeActionFromCmd(varName, firstCmd)
			if t.insertActionBeforeRef(t.tree.Root, ref, newAction) == false {
				panic("reference node not found")
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
					if t.insertActionBeforeRef(t.tree.Root, ref, newAction) == false {
						panic("reference node not found")
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
					panic("pipe not found in cmd")
				}
				newAction := createAVariablePipeAction(varName, pipeToMove)
				if t.insertActionBeforeRef(t.tree.Root, ref, newAction) == false {
					panic("reference node not found")
				}
				return true
			}
		}
	}

	return false
}

// insertActionBeforeRef browses given node list until it can find ref node,
// it then insert newAction before the ref node.
// It returns false if it failed to insert the new node.
func (t *treeSimplifier) insertActionBeforeRef(list *parse.ListNode, ref parse.Node, newAction *parse.ActionNode) bool {
	for i := 0; i < len(list.Nodes); i++ {
		if list.Nodes[i] == ref {
			list.Nodes = append(list.Nodes, nil)
			copy(list.Nodes[i+1:], list.Nodes[i:])
			list.Nodes[i] = newAction
			return true
		}
		switch node := list.Nodes[i].(type) {
		case *parse.IfNode:
			if node.List != nil && t.insertActionBeforeRef(node.List, ref, newAction) {
				return true
			}
			if node.ElseList != nil && t.insertActionBeforeRef(node.ElseList, ref, newAction) {
				return true
			}
		case *parse.RangeNode:
			if node.List != nil && t.insertActionBeforeRef(node.List, ref, newAction) {
				return true
			}
			if node.ElseList != nil && t.insertActionBeforeRef(node.ElseList, ref, newAction) {
				return true
			}
		case *parse.WithNode:
			if node.List != nil && t.insertActionBeforeRef(node.List, ref, newAction) {
				return true
			}
			if node.ElseList != nil && t.insertActionBeforeRef(node.ElseList, ref, newAction) {
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
// this func will create: {{$name := up "what"}}
func createAVariablePipeActionFromCmd(name string, cmd *parse.CommandNode) *parse.ActionNode {
	varNode := &parse.VariableNode{
		NodeType: parse.NodeVariable,
		Ident:    []string{name},
	}
	actionPipe := &parse.PipeNode{}
	actionPipe.Decl = append(actionPipe.Decl, varNode)
	actionPipe.Cmds = append(actionPipe.Cmds, cmd)
	node := &parse.ActionNode{
		NodeType: parse.NodeAction,
		Pipe:     actionPipe,
	}
	return node
}

// createAVariableNode creates a VariableNode with givne name.
func createAVariableNode(name string) *parse.VariableNode {
	varNode := &parse.VariableNode{
		NodeType: parse.NodeVariable,
		Ident:    []string{name},
	}
	return varNode
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

// getPipePrecedingIdentifier identifies a PipeNode preceding an IdentifierNode within given pipe.Cmds.
// example:
// {{("some" |lower) | up}}
// pipe is: ("some" |lower)
// identifier is: up
func getPipePrecedingIdentifier(node *parse.PipeNode) (*parse.CommandNode, *parse.PipeNode) {
	if len(node.Cmds) > 1 {
		for i, cmd := range node.Cmds {
			if pipe, ok := cmd.Args[0].(*parse.PipeNode); ok {
				if len(node.Cmds) > i+1 {
					if _, okk := node.Cmds[i+1].Args[0].(*parse.IdentifierNode); okk {
						return cmd, pipe
					}
				}
			}
		}
	}
	return nil, nil
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