//
// Copyright 2022 Sean C Foley
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tree

import (
	"strconv"
	"strings"
)

// Path is a list of nodes derived from a tree.
// Each node in the list corresponds to a node in the tree.
// Each corresponding node is a direct or indirect sub-node of the previous corresponding node in the list.
//
// In other words, a path follows a pathway through a tree from the root to a sub-node, possibly all the way to a leaf.
// Not all nodes in the pathway need to included in the linked list.  Regardless, each node is a direct or indirect sub-node of the previous.
type Path struct {
	root, leaf *PathNode
}

// GetRoot returns the beginning of the Path, which may or may not match the tree root of the originating tree.
func (path *Path) GetRoot() *PathNode {
	return path.root
}

// GetLeaf returns the end of the Path, which may or may not match a leaf in the originating tree.
func (path *Path) GetLeaf() *PathNode {
	return path.leaf
}

// String returns a visual representation of the Path with one node per line.
func (path *Path) String() string {
	return path.ListString(true)
}

// ListString returns a visual representation of the tree with one node per line, with or without the non-added keys.
func (path *Path) ListString(withNonAddedKeys bool) string {
	return path.GetRoot().ListString(withNonAddedKeys, true)
}

// PathNode is an element in the list of a Path
type PathNode struct {
	previous, next *PathNode

	// the key for the node
	item E

	// only for associative trie nodes
	value V //TODO LATER generics - this could be a generic, but to be consistent with clone, need to be careful about copies/clones, maybe consistent with interfaces (this might not be necessary)

	// the number of added nodes below this one, including this one if added
	storedSize int

	added bool
}

func (node *PathNode) Next() *PathNode {
	return node.next
}

func (node *PathNode) Previous() *PathNode {
	return node.previous
}

func (node *PathNode) getKey() (key E) {
	if node != nil {
		key = node.item
	}
	return
}

// GetKey gets the key used for placing the node in the tree.
func (node *PathNode) GetKey() E {
	return node.getKey()
}

// GetValue returns the value assigned to the node
func (node *PathNode) GetValue() (val V) {
	if node != nil {
		val = node.value
	}
	return
}

// IsAdded returns whether the node was "added".
// Some binary tree nodes are considered "added" and others are not.
// Those nodes created for key elements added to the tree are "added" nodes.
// Those that are not added are those nodes created to serve as junctions for the added nodes.
// Only added elements contribute to the size of a tree.
// When removing nodes, non-added nodes are removed automatically whenever they are no longer needed,
// which is when an added node has less than two added sub-nodes.
func (node *PathNode) IsAdded() bool {
	return node != nil && node.added
}

// Size returns the count of nodes added to the sub-tree starting from this node as root and moving downwards to sub-nodes.
// This is a constant-time operation since the size is maintained in each node and adjusted with each add and Remove operation in the sub-tree.
func (node *PathNode) Size() (storedSize int) {
	if node != nil {
		storedSize = node.storedSize
		if storedSize == sizeUnknown {
			prev, next := node, node.next
			for ; next != nil && next.storedSize == sizeUnknown; prev, next = next, next.next {
			}
			var nodeSize int
			for {
				if prev.IsAdded() {
					nodeSize++
				}
				if next != nil {
					nodeSize += next.storedSize
				}
				prev.storedSize = nodeSize
				if prev == node {
					break
				}
				prev = node.previous
			}
			storedSize = node.storedSize
		}
	}
	return
}

// Returns a visual representation of this node including the key, with an open circle indicating this node is not an added node,
// a closed circle indicating this node is an added node.
func (node *PathNode) String() string {
	if node == nil {
		return nodeString(nil)
	}
	return nodeString(node)
}

// ListString returns a visual representation of the sub-list with this node as root, with one node per line.
//
// withNonAddedKeys: whether to show nodes that are not added nodes
// withSizes: whether to include the counts of added nodes in each sub-list
func (node *PathNode) ListString(withNonAddedKeys, withSizes bool) string {
	builder := strings.Builder{}
	builder.WriteByte('\n')
	node.printList(&builder, indents{}, withNonAddedKeys, withSizes)
	return builder.String()
}

func (node *PathNode) printList(builder *strings.Builder,
	indents indents,
	withNonAdded,
	withSizes bool) {
	if node == nil {
		builder.WriteString(indents.nodeIndent)
		builder.WriteString(nilString())
		builder.WriteByte('\n')
		return
	}
	next := node
	for {
		if withNonAdded || next.IsAdded() {
			builder.WriteString(indents.nodeIndent)
			builder.WriteString(next.String())
			if withSizes {
				builder.WriteString(" (")
				builder.WriteString(strconv.Itoa(next.Size()))
				builder.WriteByte(')')
			}
			builder.WriteByte('\n')
		} else {
			builder.WriteString(indents.nodeIndent)
			builder.WriteString(nonAddedNodeCircle)
			builder.WriteByte('\n')
		}
		indents.nodeIndent = indents.subNodeInd + rightElbow
		indents.subNodeInd = indents.subNodeInd + belowElbows
		if next = next.next; next == nil {
			break
		}
	}
}
