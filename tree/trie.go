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
	"fmt"
	"strings"
	"unsafe"
)

// BinTrie is a binary trie.
//
// To use BinTrie, your keys implement TrieKey.
//
// All keys are either fixed, in which the key value does not change,
// or comprising of a prefix in which an initial sequence of bits does not change, and the the remaining bits represent all bit values.
// The length of the initial fixed sequence of bits is the prefix length.
// The total bit length is the same for all keys.
//
// A key with a prefix is also known as a prefix block, and represents all bit sequences with the same prefix.
//
// The zero value for BinTrie is a binary trie ready for use.
//
// Each node can be associated with a value, making BinTrie an associative binary trie.
// If you do not wish to associate values to nodes, then use the type EmptyValueType,
// in which case the value will be ignored in functions that print node strings.
type BinTrie[E TrieKey[E], V any] struct {
	binTree[E, V]
}

type EmptyValueType struct{}

func (trie *BinTrie[E, V]) toBinTree() *binTree[E, V] {
	return (*binTree[E, V])(unsafe.Pointer(trie))
}

// GetRoot returns the root of this trie (in the case of bounded tries, this would be the bounded root)
func (trie *BinTrie[E, V]) GetRoot() (root *BinTrieNode[E, V]) {
	if trie != nil {
		//root = trie.root.toTrieNode()
		root = toTrieNode(trie.root)
	}
	return
}

// Returns the root of this trie (in the case of bounded tries, the absolute root ignores the bounds)
func (trie *BinTrie[E, V]) absoluteRoot() (root *BinTrieNode[E, V]) {
	if trie != nil {
		//root = trie.root.toTrieNode()
		root = toTrieNode(trie.root)
	}
	return
}

// Size returns the number of elements in the tree.
// Only nodes for which IsAdded() returns true are counted.
// When zero is returned, IsEmpty() returns true.
func (trie *BinTrie[E, V]) Size() int {
	return trie.toBinTree().Size()
}

// NodeSize returns the number of nodes in the tree, which is always more than the number of elements.
func (trie *BinTrie[E, V]) NodeSize() int {
	return trie.toBinTree().NodeSize()
}

func (trie *BinTrie[E, V]) ensureRoot(key E) *BinTrieNode[E, V] {
	root := trie.root
	if root == nil {
		root = trie.setRoot(key.ToPrefixBlockLen(0))
	}
	//return root.toTrieNode()
	return toTrieNode(root)
}

func (trie *BinTrie[E, V]) setRoot(key E) *binTreeNode[E, V] {
	root := &binTreeNode[E, V]{
		item:     key,
		cTracker: &changeTracker{},
	}
	root.setAddr()
	trie.root = root
	return root
}

// Iterator returns an iterator that iterates through the elements of the sub-tree with this node as the root.
// The iteration is in sorted element order.
func (trie *BinTrie[E, V]) Iterator() TrieKeyIterator[E] {
	return trie.GetRoot().Iterator()
}

// DescendingIterator returns an iterator that iterates through the elements of the subtrie with this node as the root.
// The iteration is in reverse sorted element order.
func (trie *BinTrie[E, V]) DescendingIterator() TrieKeyIterator[E] {
	return trie.GetRoot().DescendingIterator()
}

// String returns a visual representation of the tree with one node per line.
func (trie *BinTrie[E, V]) String() string {
	if trie == nil {
		return nilString()
	}
	return trie.binTree.String()
}

// TreeString returns a visual representation of the tree with one node per line, with or without the non-added keys.
func (trie *BinTrie[E, V]) TreeString(withNonAddedKeys bool) string {
	if trie == nil {
		return "\n" + nilString()
	}
	return trie.binTree.TreeString(withNonAddedKeys)
}

type SubNodesMapping[E TrieKey[E], V any] struct {
	val V

	// subNodes is a []*BinTrieNode from the same trie of type BinTrie[E, SubNodesMapping[V]] TODO remove
	// That makes it recursive.  The node value here points to nodes with the same value.
	// So is why we use the type "any".  We could have used []any but it's better to use just one interface per node.
	//subNodes any

	// subNodes is the list of direct and indirect added subnodes in the original trie
	subNodes []*BinTrieNode[E, AddedSubnodeMapping]
}

type AddedSubnodeMapping any // AddedSubnodeMapping / any is always SubNodesMapping[E,V]

type printWrapper[E TrieKey[E], V any] struct {
	*BinTrieNode[E, AddedSubnodeMapping]
}

func (p printWrapper[E, V]) GetValue() V {
	nodeValue := p.BinTrieNode.GetValue()
	if nodeValue == nil {
		var v V
		return v
	}
	return nodeValue.(SubNodesMapping[E, V]).val
}

// ConstructAddedNodesTree provides an associative trie in which the root and each added node of this trie are mapped to a list of their respective direct added sub-nodes.
// This trie provides an alternative non-binary tree structure of the added nodes.
// It is used by ToAddedNodesTreeString to produce a string showing the alternative structure.
// If there are no non-added nodes in this trie,
// then the alternative tree structure provided by this method is the same as the original trie.
// The trie values of this trie are of type []*BinTrieNode
func (trie *BinTrie[E, V]) ConstructAddedNodesTree() BinTrie[E, AddedSubnodeMapping] {
	newRoot := &binTreeNode[E, AddedSubnodeMapping]{
		item:     trie.root.item,
		cTracker: &changeTracker{},
	}
	newRoot.setAddr()
	if trie.root.IsAdded() {
		newRoot.SetAdded()
	}
	emptyTrie := BinTrie[E, AddedSubnodeMapping]{binTree[E, AddedSubnodeMapping]{newRoot}}

	// populate the keys from the original trie into the new trie
	AddTrieKeys(&emptyTrie, trie.absoluteRoot())

	// now, as we iterate,
	// we find our parent and add ourselves to that parent's list of subnodes

	var cachingIterator CachingTrieNodeIterator[E, AddedSubnodeMapping]
	cachingIterator = emptyTrie.ContainingFirstAllNodeIterator(true)
	thisIterator := trie.ContainingFirstAllNodeIterator(true)
	var newNext *BinTrieNode[E, AddedSubnodeMapping]
	var thisNext *BinTrieNode[E, V]
	for newNext, thisNext = cachingIterator.Next(), thisIterator.Next(); newNext != nil; newNext, thisNext = cachingIterator.Next(), thisIterator.Next() {

		// populate the values from the original trie into the new trie
		newNext.SetValue(SubNodesMapping[E, V]{val: thisNext.GetValue()})

		cachingIterator.CacheWithLowerSubNode(newNext)
		cachingIterator.CacheWithUpperSubNode(newNext)

		// the cached object is our parent
		if newNext.IsAdded() {
			var parent *BinTrieNode[E, AddedSubnodeMapping]
			parent = cachingIterator.GetCached().(*BinTrieNode[E, AddedSubnodeMapping])

			if parent != nil {
				// find added parent, or the root if no added parent
				for !parent.IsAdded() {
					parentParent := parent.GetParent()
					if parentParent == nil {
						break
					}
					parent = parentParent
				}
				// store ourselves with that added parent or root
				var val SubNodesMapping[E, V]
				val = parent.GetValue().(SubNodesMapping[E, V])
				var list []*BinTrieNode[E, AddedSubnodeMapping]
				if val.subNodes == nil {
					list = make([]*BinTrieNode[E, AddedSubnodeMapping], 0, 3)
				} else {
					list = val.subNodes
				}
				val.subNodes = append(list, newNext)
				parent.SetValue(val)
			} // else root
		}
	}
	return emptyTrie
}

type indentsNode[E TrieKey[E]] struct {
	inds indents
	node *BinTrieNode[E, AddedSubnodeMapping]
}

// AddedNodesTreeString provides a flattened version of the trie showing only the contained added nodes and their containment structure, which is non-binary.
// The root node is included, which may or may not be added.
func (trie *BinTrie[E, V]) AddedNodesTreeString() string {
	if trie == nil {
		return "\n" + nilString()
	}

	addedTree := trie.ConstructAddedNodesTree()
	var stack []indentsNode[E]
	var root, nextNode *BinTrieNode[E, AddedSubnodeMapping]
	root = addedTree.absoluteRoot()
	builder := strings.Builder{}
	builder.WriteByte('\n')
	nodeIndent, subNodeIndent := "", ""
	nextNode = root
	for {
		builder.WriteString(nodeIndent)
		builder.WriteString(nodeString[E, V](printWrapper[E, V]{nextNode}))
		builder.WriteByte('\n')

		var nextVal AddedSubnodeMapping // SubNodesMapping[E, V]
		nextVal = nextNode.GetValue()
		var nextNodes []*BinTrieNode[E, AddedSubnodeMapping]
		//var nextNodes []*BinTrieNode[E, SubNodesMapping[E, V]]
		if nextVal != nil {
			mapping := nextVal.(SubNodesMapping[E, V])
			if mapping.subNodes != nil {
				nextNodes = mapping.subNodes
			}
		}
		if len(nextNodes) > 0 {
			i := len(nextNodes) - 1
			lastIndents := indents{
				nodeIndent: subNodeIndent + rightElbow,
				subNodeInd: subNodeIndent + belowElbows,
			}

			var nNode *BinTrieNode[E, AddedSubnodeMapping] // SubNodesMapping[E, V]
			nNode = nextNodes[i]
			if stack == nil {
				stack = make([]indentsNode[E], 0, addedTree.Size())
			}
			//next := nNode
			stack = append(stack, indentsNode[E]{lastIndents, nNode})
			if len(nextNodes) > 1 {
				firstIndents := indents{
					nodeIndent: subNodeIndent + leftElbow,
					subNodeInd: subNodeIndent + inBetweenElbows,
				}
				for i--; i >= 0; i-- {
					nNode = nextNodes[i]
					//next =  nNode;
					stack = append(stack, indentsNode[E]{firstIndents, nNode})
				}
			}
		}
		stackLen := len(stack)
		if stackLen == 0 {
			break
		}
		newLen := stackLen - 1
		nextItem := stack[newLen]
		stack = stack[:newLen]
		nextNode = nextItem.node
		nextIndents := nextItem.inds
		nodeIndent = nextIndents.nodeIndent
		subNodeIndent = nextIndents.subNodeInd
	}
	return builder.String()
}

//// ConstructAddedNodesTree provides an associative trie in which the root and each added node of this trie are mapped to a list of their respective direct added sub-nodes.
//// This trie provides an alternative non-binary tree structure of the added nodes.
//// It is used by ToAddedNodesTreeString to produce a string showing the alternative structure.
//// If there are no non-added nodes in this trie,
//// then the alternative tree structure provided by this method is the same as the original trie.
//// The trie values of this trie are of type []*BinTrieNode
//func (trie *BinTrie[E, V]) ConstructAddedNodesTree() BinTrie[E, SubNodesMapping[E, V]] {
//	newRoot := &binTreeNode[E, SubNodesMapping[E, V]]{
//		item:     trie.root.item,
//		cTracker: &changeTracker{},
//	}
//	newRoot.setAddr()
//	if trie.root.IsAdded() {
//		newRoot.SetAdded()
//	}
//	emptyTrie := BinTrie[E, SubNodesMapping[E, V]]{binTree[E, SubNodesMapping[E, V]]{newRoot}}
//
//	// this trie's structure goes into the new trie, but without the values
//	AddTrieKeys(&emptyTrie, trie.absoluteRoot())
//
//	// now, as we iterate,
//	// we find our parent and add ourselves to that parent's list of subnodes
//
//	var cachingIterator CachingTrieNodeIterator[E, SubNodesMapping[E, V]]
//	cachingIterator = emptyTrie.ContainingFirstAllNodeIterator(true)
//	thisIterator := trie.ContainingFirstAllNodeIterator(true)
//	var next *BinTrieNode[E, SubNodesMapping[E, V]]
//	var thisNext *BinTrieNode[E, V]
//	for next, thisNext = cachingIterator.Next(), thisIterator.Next(); next != nil; next, thisNext = cachingIterator.Next(), thisIterator.Next() {
//
//		// populate the values from the original trie into the new trie
//		next.SetValue(SubNodesMapping[E, V]{val: thisNext.GetValue()})
//
//		cachingIterator.CacheWithLowerSubNode(next)
//		cachingIterator.CacheWithUpperSubNode(next)
//
//		// the cached object is our parent
//		if next.IsAdded() {
//			var parent *BinTrieNode[E, SubNodesMapping[E, V]]
//			parent = cachingIterator.GetCached().(*BinTrieNode[E, SubNodesMapping[E, V]])
//			//parent := cachingIterator.GetCached().(*BinTrieNode)
//			if parent != nil {
//				// find added parent, or the root if no added parent
//				for !parent.IsAdded() {
//					parentParent := parent.GetParent()
//					if parentParent == nil {
//						break
//					}
//					parent = parentParent
//				}
//				// store ourselves with that added parent or root
//				var val SubNodesMapping[E, V]
//				val = parent.GetValue()
//				var list []*BinTrieNode[E, SubNodesMapping[E, V]]
//				if val.subNodes == nil {
//					list = make([]*BinTrieNode[E, SubNodesMapping[E, V]], 0, 3)
//				} else {
//					list = val.subNodes
//					//list = val.subNodes.([]*BinTrieNode[E, SubNodesMapping[V]])
//				}
//				//list = append(list, next)
//				val.subNodes = append(list, next)
//				parent.SetValue(val)
//			} // else root
//		}
//	}
//	return emptyTrie
//}
//
//// AddedNodesTreeString provides a flattened version of the trie showing only the contained added nodes and their containment structure, which is non-binary.
//// The root node is included, which may or may not be added.
//func (trie *BinTrie[E, V]) AddedNodesTreeString() string {
//	if trie == nil {
//		return "\n" + nilString()
//	}
//	type indentsNode struct {
//		inds indents
//		node *BinTrieNode[E, SubNodesMapping[E, V]]
//	}
//	//var stack list.List
//
//	/*
//																		https://go.googlesource.com/proposal/+/refs/heads/master/design/43651-type-parameters.md#generic-types
//
//																					"A generic type can refer to itself in cases where a type can
//																					ordinarily refer to itself, but when it does so the type arguments
//																					must be the type parameters, listed in the same order. This
//																					restriction prevents infinite recursion of type instantiation."
//
//																			./trie.go:40:28: instantiation cycle:
//																				./trie.go:237:27: V instantiated as SubNodesMapping[E, V]
//
//																	https://go.dev/play/p/ueO6Zo9OCf4
//
//																	Not clear what to do, other than use interfaces
//
//																I really do need to map to a single type, and that type, it does not to include both the value of the current trie, and the set of subnodes
//
//															The worst thing about using an interface is that this trie is what we return from the COnstructAddedNodes method
//															So it would be nice to avoid the interface
//														How about we make it into a new type somehow?
//													Internally we have the existing structure, but externally we provide methods?
//												GetSubnodes()
//												hmmmm how about we skip the E and then we use "any" for the BinTrieNode?
//											That works because we do not really even need the E
//
//										But then AddTrieKeys will not work, it needs the keys to be same type.
//										I guess you could make a variant of AddTrieKeys for "any" as the key for the target
//									But then, can we use "any" as a trie key?  Nah.
//
//								I could possibly redefine BinTrie to take E as a parameter for the value too.  nah, that doesn't help.
//
//							OK, this is not that thing above, this is any cycle at all is bad.
//							So I cannot take V from BinTrie and then make it a part of any further instantiation of another BinTrie.
//					I guess it would have to be BinTrie[E,V] and nothing else.
//
//				OK, what I want is a new type with methods to represent this thing.
//				type AddedSubnodeMapping struct {
//		contents any // this is the SubNodesMapping[E, V] with no changes to it
//				}
//	*/
//	//var addedTree BinTrie[E, SubNodesMapping[E, V]]
//	addedTree := trie.ConstructAddedNodesTree()
//	var stack []indentsNode
//	var root, nextNode *BinTrieNode[E, SubNodesMapping[E, V]]
//	root = addedTree.absoluteRoot()
//	builder := strings.Builder{}
//	builder.WriteByte('\n')
//	nodeIndent, subNodeIndent := "", ""
//	nextNode = root
//	for {
//		builder.WriteString(nodeIndent)
//		builder.WriteString(nodeString[E, V](printWrapper[E, V]{nextNode}))
//
//		//xxxx
//		//if nextNode.IsAdded() {
//		//	builder.WriteString(addedNodeCircle)
//		//} else {
//		//	builder.WriteString(nonAddedNodeCircle)
//		//}
//		//builder.WriteByte(' ')
//		//builder.WriteString(fmt.Sprint(nextNode.GetKey())) xxxxxxxxxx see nodeString xxxxxx
//		//xxxx
//
//		builder.WriteByte('\n')
//		var nextVal SubNodesMapping[E, V]
//		nextVal = nextNode.GetValue()
//		var nextNodes []*BinTrieNode[E, SubNodesMapping[E, V]]
//		if nextVal.subNodes != nil {
//			nextNodes = nextVal.subNodes
//			//nextNodes = nextVal.([]*BinTrieNode)
//		}
//		if nextNodes != nil && len(nextNodes) > 0 {
//			i := len(nextNodes) - 1
//			lastIndents := indents{
//				nodeIndent: subNodeIndent + rightElbow,
//				subNodeInd: subNodeIndent + belowElbows,
//			}
//			var nNode *BinTrieNode[E, SubNodesMapping[E, V]]
//			nNode = nextNodes[i]
//			if stack == nil {
//				stack = make([]indentsNode, 0, addedTree.Size())
//			}
//			//next := nNode
//			stack = append(stack, indentsNode{lastIndents, nNode})
//			if len(nextNodes) > 1 {
//				firstIndents := indents{
//					nodeIndent: subNodeIndent + leftElbow,
//					subNodeInd: subNodeIndent + inBetweenElbows,
//				}
//				for i--; i >= 0; i-- {
//					nNode = nextNodes[i]
//					//next =  nNode;
//					stack = append(stack, indentsNode{firstIndents, nNode})
//				}
//			}
//		}
//		//if stack == nil {
//		//	break
//		//}
//		stackLen := len(stack)
//		if stackLen == 0 {
//			break
//		}
//		newLen := stackLen - 1
//		nextItem := stack[newLen]
//		stack = stack[:newLen]
//		nextNode = nextItem.node
//		nextIndents := nextItem.inds
//		nodeIndent = nextIndents.nodeIndent
//		subNodeIndent = nextIndents.subNodeInd
//	}
//	return builder.String()
//}

//// ConstructAddedNodesTree provides an associative trie in which the root and each added node of this trie are mapped to a list of their respective direct added sub-nodes.
//// This trie provides an alternative non-binary tree structure of the added nodes.
//// It is used by ToAddedNodesTreeString to produce a string showing the alternative structure.
//// If there are no non-added nodes in this trie,
//// then the alternative tree structure provided by this method is the same as the original trie.
//// The trie values of this trie are of type []*BinTrieNode
//func (trie *BinTrie[E, V]) ConstructAddedNodesTree() BinTrie[E, []*BinTrieNode[E, V]] {
//	newRoot := &binTreeNode[E, []*BinTrieNode[E, V]]{
//		item:     trie.root.item,
//		cTracker: &changeTracker{},
//	}
//	newRoot.setAddr()
//	if trie.root.IsAdded() {
//		newRoot.SetAdded()
//	}
//	emptyTrie := BinTrie[E, []*BinTrieNode[E, V]]{binTree[E, []*BinTrieNode[E, V]]{newRoot}}
//	//
//	// type *"github.com/seancfoley/bintree/tree".binTreeNode[E, V]) as the
//	// type *"github.com/seancfoley/bintree/tree".binTreeNode[E, V]
//	//
//	//AssociativeAddressTrie<E, ? extends List<? extends AssociativeTrieNode<E, ?>>> emptyTrie;
//	// // returns AssociativeAddressTrie<E, ? extends List<? extends AssociativeTrieNode<E, ?>>> of keys mapped to subnodes
//	emptyTrie.Clear()
//
//	//TODO in the original the addTrie calls the variant that adds only keys!  Ignores values.  But maybe we copy over the values anyway?  Quite possibly.
//	// So that is why look like *BinTrieNode[E, V] and not *BinTrieNode[E, []*BinTrieNode[E, V]]
//	// So here I'd say maybe we need to iterate over both at the same time.
//	// We set the values on the parent, and we use the caching to access the parent.
//	// So, the node we assign to the parent should come from the other trie!  It should match the node for emptyTrie.
//
//	AddTrieKeys(&emptyTrie, trie.absoluteRoot()) // TODO here xxx we need an addTrie in which values are not necessarily same type xxxxx
//	//xxx next step is the generic addTrie for different trie values xxxx
//
//	// this trie goes into the new trie, then as we iterate,
//	// we find our parent and add ourselves to that parent's list of subnodes
//
//	//var cachingIterator CachingTrieNodeIterator[E, V]
//	//cachingIterator = trie.ContainingFirstAllNodeIterator(true)
//
//	var cachingIterator CachingTrieNodeIterator[E, []*BinTrieNode[E, V]]
//	cachingIterator = emptyTrie.ContainingFirstAllNodeIterator(true)
//	thisIterator := trie.ContainingFirstAllNodeIterator(true)
//	var next *BinTrieNode[E, []*BinTrieNode[E, V]]
//	var thisNext *BinTrieNode[E, V]
//	for next, thisNext = cachingIterator.Next(), thisIterator.Next(); next != nil; next, thisNext = cachingIterator.Next(), thisIterator.Next() {
//		cachingIterator.CacheWithLowerSubNode(next)
//		cachingIterator.CacheWithUpperSubNode(next)
//
//		// the cached object is our parent
//		if next.IsAdded() {
//			parent := cachingIterator.GetCached().(*BinTrieNode[E, []*BinTrieNode[E, V]])
//			if parent != nil {
//				// find added parent, or the root if no added parent
//				// this part would be tricky if we accounted for the bounds,
//				// maybe we'd have to filter on the bounds, and also look for the sub-root
//				for !parent.IsAdded() {
//					parentParent := parent.GetParent()
//					if parentParent == nil {
//						break
//					}
//					parent = parentParent
//				}
//				// store ourselves with that added parent or root
//				val := parent.GetValue() //xxxx we need to get the parent from the secondary trie when setting and getting the value, this is from original trie - but we would need secondary trie to duplicate shape of original
//				var list []*BinTrieNode[E, V]
//				if val == nil {
//					list = make([]*BinTrieNode[E, V], 0, 3)
//				} else {
//					list = val //.([]*BinTrieNode[E, V])
//				}
//				list = append(list, thisNext)
//				parent.SetValue(list) //xxxxx
//			} // else root
//		}
//	}
//	return emptyTrie
//}
//
//// AddedNodesTreeString provides a flattened version of the trie showing only the contained added nodes and their containment structure, which is non-binary.
//// The root node is included, which may or may not be added.
//func (trie *BinTrie[E, V]) AddedNodesTreeString() string {
//	if trie == nil {
//		return "\n" + nilString()
//	}
//	// TODO UGH the tree value is recursive!!!  Each value points to more nodes, whose values point to more nodes...
//	// For one thing, this wipes out the original tree values of course
//	// For the other thing, such a trie cannot be described by generics, or can it?
//	// BinTree[K, V RecursiveValue[K, V]]  where RecrusiveValue is a list of BinTrieNode[K, V RecursiveValue[K, V]]
//	// Now, I could fix this by using BinTrie[E, any] and go back to the original code above
//	// But then, we'd lost all the original values
//	// I think maybe a combination?  We want each value to point to the list of sub nodes in the same trie
//	// But we'd also want it to point to its original value
//	// We want both!  What we have now is wrong.
//	// The book-keeping for both is tricky for sure.
//	// in fact it will be just struct { V, any }
//	// And so, what does that mean above, do we still need two iterators?  I think so.
//	// We certainly need access to both tries, and without using duplicate iterators, we'd need to do lookups all the time.
//	// In one iterator we have the original trie node, the other is the node with the funky values.
//	// Either iterator could use caching, but it's the funky value iterator that needs to locate the parent.
//	// I guess there is just one other complication, while iterating we need to copy the value over from the orig trie node to the funky value node.
//	// And so, the Java code will not change all that much.  The java code is correct that there is an "any" component to the values.
//
//	type indentsNode struct {
//		inds indents
//		node *BinTrieNode[E, []*BinTrieNode[E, V]]
//	}
//	//var stack list.List
//	var addedTree BinTrie[E, []*BinTrieNode[E, V]]
//	addedTree = trie.ConstructAddedNodesTree()
//	var stack []indentsNode
//	var root *BinTrieNode[E, []*BinTrieNode[E, V]]
//	root = addedTree.absoluteRoot()
//	builder := strings.Builder{}
//	builder.WriteByte('\n')
//	nodeIndent, subNodeIndent := "", ""
//	nextNode := root
//	for {
//		builder.WriteString(nodeIndent)
//		if nextNode.IsAdded() {
//			builder.WriteString(addedNodeCircle)
//		} else {
//			builder.WriteString(nonAddedNodeCircle)
//		}
//		builder.WriteByte(' ')
//		builder.WriteString(fmt.Sprint(nextNode.GetKey()))
//		builder.WriteByte('\n')
//		var nextVal []*BinTrieNode[E, V]
//		nextVal = nextNode.GetValue()
//		var nextNodes []*BinTrieNode[E, V]
//		if nextVal != nil {
//			nextNodes = nextVal
//			//nextNodes = nextVal.([]*BinTrieNode) //TODO this is blowing up today for associative: panic: interface conversion: tree.V is int, not []*tree.BinTrieNode
//			// we had emptyTrie.addTrie(trie.absoluteRoot(), true) which copied the values and we were using any for the values so there was int copying that was probably not overridden for leaf nodes
//		}
//		if len(nextNodes) > 0 {
//			i := len(nextNodes) - 1
//			lastIndents := indents{
//				nodeIndent: subNodeIndent + rightElbow,
//				subNodeInd: subNodeIndent + belowElbows,
//			}
//			var nNode *BinTrieNode[E, V]
//			nNode = nextNodes[i]
//			if stack == nil {
//				stack = make([]indentsNode, 0, addedTree.Size())
//			}
//			//next := nNode
//			stack = append(stack, indentsNode{lastIndents, nNode})
//			if len(nextNodes) > 1 {
//				firstIndents := indents{
//					nodeIndent: subNodeIndent + leftElbow,
//					subNodeInd: subNodeIndent + inBetweenElbows,
//				}
//				for i--; i >= 0; i-- {
//					nNode = nextNodes[i]
//					//next =  nNode;
//					stack = append(stack, indentsNode{firstIndents, nNode})
//				}
//			}
//		}
//		if stack == nil {
//			break
//		}
//		stackLen := len(stack)
//		if stackLen == 0 {
//			break
//		}
//		newLen := stackLen - 1
//		nextItem := stack[newLen]
//		stack = stack[:newLen]
//		nextNode = nextItem.node
//		nextIndents := nextItem.inds
//		nodeIndent = nextIndents.nodeIndent
//		subNodeIndent = nextIndents.subNodeInd
//	}
//	return builder.String()
//}

// ConstructAddedNodesTree provides an associative trie in which the root and each added node of this trie are mapped to a list of their respective direct added sub-nodes.
// This trie provides an alternative non-binary tree structure of the added nodes.
// It is used by ToAddedNodesTreeString to produce a string showing the alternative structure.
// If there are no non-added nodes in this trie,
// then the alternative tree structure provided by this method is the same as the original trie.
// The trie values of this trie are of type []*BinTrieNode[E, V]
//func (trie *BinTrie) ConstructAddedNodesTree() BinTrie {
//	newRoot := trie.root.clone()
//	newRoot.ClearValue()
//	newRoot.cTracker = &changeTracker{}
//	emptyTrie := BinTrie{binTree{newRoot}}
//	//AssociativeAddressTrie<E, ? extends List<? extends AssociativeTrieNode<E, ?>>> emptyTrie;
//	// // returns AssociativeAddressTrie<E, ? extends List<? extends AssociativeTrieNode<E, ?>>> of keys mapped to subnodes
//	emptyTrie.Clear()
//	emptyTrie.addTrie(trie.absoluteRoot(), true)
//
//	// this trie goes into the new trie, then as we iterate,
//	// we find our parent and add ourselves to that parent's list of subnodes
//
//	cachingIterator := emptyTrie.ContainingFirstAllNodeIterator(true)
//	for next := cachingIterator.Next(); next != nil; next = cachingIterator.Next() {
//		cachingIterator.CacheWithLowerSubNode(next)
//		cachingIterator.CacheWithUpperSubNode(next)
//
//		// the cached object is our parent
//		if next.IsAdded() {
//			parent := cachingIterator.GetCached().(*BinTrieNode)
//			if parent != nil {
//				// find added parent, or the root if no added parent
//				// this part would be tricky if we accounted for the bounds,
//				// maybe we'd have to filter on the bounds, and also look for the sub-root
//				for !parent.IsAdded() {
//					parentParent := parent.GetParent()
//					if parentParent == nil {
//						break
//					}
//					parent = parentParent
//				}
//				// store ourselves with that added parent or root
//				val := parent.GetValue()
//				var list []*BinTrieNode
//				if val == nil {
//					list = make([]*BinTrieNode, 0, 3)
//				} else {
//					list = val.([]*BinTrieNode)
//				}
//				list = append(list, next)
//				parent.SetValue(list)
//			} // else root
//		}
//	}
//	return emptyTrie
//}

//// ConstructAddedNodesTree provides an associative trie in which the root and each added node of this trie are mapped to a list of their respective direct added sub-nodes.
//// This trie provides an alternative non-binary tree structure of the added nodes.
//// It is used by ToAddedNodesTreeString to produce a string showing the alternative structure.
//// If there are no non-added nodes in this trie,
//// then the alternative tree structure provided by this method is the same as the original trie.
//// The trie values of this trie are of type []*BinTrieNode[E, V]
//func (trie *BinTrie[E, V]) ConstructAddedNodesTree() BinTrie[E, []*BinTrieNode[E, V]] {
//	newRoot := trie.root.clone()
//	newRoot.ClearValue()
//	newRoot.cTracker = &changeTracker{}
//
//	//xxxxx yes, we are constructing a trie with key E, but value is not V, it is []*BinTrieNode[E, V], the list of sub-nodes of the parent, ie the added sub-nodes, so there may be more than just two xxxxx
//
//	emptyTrie := BinTrie[E, []*BinTrieNode[E, V]]{binTree[E, []*BinTrieNode[E, V]]{newRoot}}
//	//emptyTrie := BinTrie[Exxx, Vxxx]{binTree[Exxxx, Vxxxx]{newRoot}}xxxxxxxxxxxxxxxxxxxxxxxxx
//	//AssociativeAddressTrie<E, ? extends List<? extends AssociativeTrieNode<E, ?>>> emptyTrie;
//	// // returns AssociativeAddressTrie<E, ? extends List<? extends AssociativeTrieNode<E, ?>>> of keys mapped to subnodes
//	emptyTrie.Clear()
//	emptyTrie.addTrie(trie.absoluteRoot(), true)
//
//	// this trie goes into the new trie, then as we iterate,
//	// we find our parent and add ourselves to that parent's list of subnodes
//
//	var cachingIterator CachingTrieNodeIterator[E, []*BinTrieNode[E, V]]
//	//xxxx iterator e,v iteraties BinTrieNode[e,v]
//	cachingIterator = emptyTrie.ContainingFirstAllNodeIterator(true)
//	//var next *BinTrieNode[E, []*BinTrieNode[E, V]]
//	var next *BinTrieNode[E, []*BinTrieNode[E, V]]
//	for next = cachingIterator.Next(); next != nil; next = cachingIterator.Next() {
//		cachingIterator.CacheWithLowerSubNode(next)
//		cachingIterator.CacheWithUpperSubNode(next)
//
//		//xxxxx what gets you is the changing value type I think xxxx
//
//		// the cached object is our parent
//		if next.IsAdded() {
//			parent := cachingIterator.GetCached().(*BinTrieNode[E, []*BinTrieNode[E, V]]) //.(*BinTrieNode[E, V])
//			if parent != nil {
//				// find added parent, or the root if no added parent
//				// this part would be tricky if we accounted for the bounds,
//				// maybe we'd have to filter on the bounds, and also look for the sub-root
//				for !parent.IsAdded() {
//					parentParent := parent.GetParent()
//					if parentParent == nil {
//						break
//					}
//					parent = parentParent
//				}
//				// store ourselves with that added parent or root
//				var val []*BinTrieNode[E, V]
//				val = parent.GetValue()
//				var list []*BinTrieNode[E, V]
//				if val == nil {
//					list = make([]*BinTrieNode[E, V], 0, 3)
//				} else {
//					list = val //.([]*BinTrieNode[E, V])
//				}
//				list = append(list, next)
//				parent.SetValue(list)
//			} // else root
//		}
//	}
//	return emptyTrie
//}

//
//type indentsNode[E TrieKey[E], V any] struct {
//	inds indents
//	node *BinTrieNode[E, []*BinTrieNode[E, V]]
//}

/*
Recursive:
If I have a tree E,V
and I map each value to a node
then each value is BinTrieNode[E,V]
BUT that makes the tree E, BinTrieNode[E,V]
But that makes each node BinTrieNode[BinTrieNode[E,V]]
But that makes the trie E, BinTrieNode[BinTrieNode[E,V]]
But that makes each value BinTrieNode[BinTrieNode[BinTrieNode[E,V]]]
This whole thing is a mind-bender!
THAT IS THE PROBLEM
You cannot obsess about the values
They will not line up with the trie type
So then what to do?
TODO The magic trie must have value "any"!!!!
The nodes can have value BinTrieNode[E,V] but you cannot then describe the trie as a generic type in relation to the nodes
It must have a simple node type, any
*/
//
//// AddedNodesTreeString provides a flattened version of the trie showing only the contained added nodes and their containment structure, which is non-binary.
//// The root node is included, which may or may not be added.
//func (trie *BinTrie[E, V]) AddedNodesTreeString() string {
//	if trie == nil {
//		return "\n" + nilString()
//	}
//
//	//var stack list.List
//	var addedTree BinTrie[E, []*BinTrieNode[E, V]]
//	addedTree = trie.ConstructAddedNodesTree()
//	var stack []indentsNode[E, V] //xxxxx each node has type *BinTrieNode[E, []*BinTrieNode[E, V]] xxxxxx
//	var root, nextNode *BinTrieNode[E, []*BinTrieNode[E, V]]
//	root = addedTree.absoluteRoot()
//	builder := strings.Builder{}
//	builder.WriteByte('\n')
//	nodeIndent, subNodeIndent := "", ""
//	nextNode = root
//	for {
//		builder.WriteString(nodeIndent)
//		if nextNode.IsAdded() {
//			builder.WriteString(addedNodeCircle)
//		} else {
//			builder.WriteString(nonAddedNodeCircle)
//		}
//		builder.WriteByte(' ')
//		builder.WriteString(fmt.Sprint(nextNode.GetKey()))
//		builder.WriteByte('\n')
//		var nextVal []*BinTrieNode[E, []*BinTrieNode[E, V]]
//		nextVal = nextNode.GetValue()
//		var nextNodes []*BinTrieNode[E, []*BinTrieNode[E, V]]
//		if nextVal != nil {
//			nextNodes = nextVal // TODO in this case our type V is in fact *BinTrieNode[E, V], or in fact a slice of them
//			//	nextNodes = nextVal.([]*BinTrieNode[E, V]) xxxx// TODO in this case our type V is in fact *BinTrieNode[E, V], or in fact a slice of them
//		}
//		if nextNodes != nil && len(nextNodes) > 0 {
//			i := len(nextNodes) - 1
//			lastIndents := indents{
//				nodeIndent: subNodeIndent + rightElbow,
//				subNodeInd: subNodeIndent + belowElbows,
//			}
//			var nNode *BinTrieNode[E, []*BinTrieNode[E, V]]
//			nNode = nextNodes[i]
//			if stack == nil {
//				stack = make([]indentsNode[E, V], 0, addedTree.Size())
//			}
//			//next := nNode
//			stack = append(stack, indentsNode[E, V]{lastIndents, nNode})
//			if len(nextNodes) > 1 {
//				firstIndents := indents{
//					nodeIndent: subNodeIndent + leftElbow,
//					subNodeInd: subNodeIndent + inBetweenElbows,
//				}
//				for i--; i >= 0; i-- {
//					nNode = nextNodes[i]
//					//next =  nNode;
//					stack = append(stack, indentsNode[E, V]{firstIndents, nNode})
//				}
//			}
//		}
//		if stack == nil {
//			break
//		}
//		stackLen := len(stack)
//		if stackLen == 0 {
//			break
//		}
//		newLen := stackLen - 1
//		nextItem := stack[newLen]
//		stack = stack[:newLen]
//		nextNode = nextItem.node
//		nextIndents := nextItem.inds
//		nodeIndent = nextIndents.nodeIndent
//		subNodeIndent = nextIndents.subNodeInd
//	}
//	return builder.String()
//}

// Add adds the given key to the trie, returning true if not there already.
func (trie *BinTrie[E, V]) Add(key E) bool {
	root := trie.ensureRoot(key)
	result := &opResult[E, V]{
		key: key,
		op:  insert,
	}
	root.matchBits(result)
	return !result.exists
}

// AddNode is similar to Add but returns the new or existing node.
func (trie *BinTrie[E, V]) AddNode(key E) *BinTrieNode[E, V] {
	root := trie.ensureRoot(key)
	result := &opResult[E, V]{
		key: key,
		op:  insert,
	}
	root.matchBits(result)
	node := result.existingNode
	if node == nil {
		node = result.inserted
	}
	return node
}

func (trie *BinTrie[E, V]) addNode(result *opResult[E, V], fromNode *BinTrieNode[E, V]) *BinTrieNode[E, V] {
	fromNode.matchBitsFromIndex(fromNode.GetKey().GetPrefixLen().Len(), result)
	node := result.existingNode
	if node == nil {
		return result.inserted
	}
	return node
}

func (trie *BinTrie[E, V]) addTrie(addedTree *BinTrieNode[E, V], withValues bool) *BinTrieNode[E, V] {
	iterator := addedTree.ContainingFirstAllNodeIterator(true)
	toAdd := iterator.Next()
	result := &opResult[E, V]{
		key: toAdd.GetKey(),
		op:  insert,
	}
	var firstNode *BinTrieNode[E, V]
	root := trie.absoluteRoot()
	firstAdded := toAdd.IsAdded()
	if firstAdded {
		if withValues {
			result.newValue = toAdd.GetValue()
			// new value assignment
		}
		firstNode = trie.addNode(result, root)
	} else {
		firstNode = root
	}
	lastAddedNode := firstNode
	for iterator.HasNext() {
		iterator.CacheWithLowerSubNode(lastAddedNode)
		iterator.CacheWithUpperSubNode(lastAddedNode)
		toAdd = iterator.Next()
		cachedNode := iterator.GetCached().(*BinTrieNode[E, V])
		if toAdd.IsAdded() {
			addrNext := toAdd.GetKey()
			result.key = addrNext
			result.existingNode = nil
			result.inserted = nil
			if withValues {
				result.newValue = toAdd.GetValue()
				// new value assignment
			}
			lastAddedNode = trie.addNode(result, cachedNode)
		} else {
			lastAddedNode = cachedNode
		}
	}
	if !firstAdded {
		firstNode = trie.GetNode(addedTree.GetKey())
	}
	return firstNode
}

// AddTrieKeys copies the trie node structure of addedTrie into trie, but does not copy node mapped values
func AddTrieKeys[E TrieKey[E], V1 any, V2 any](trie *BinTrie[E, V1], addedTree *BinTrieNode[E, V2]) *BinTrieNode[E, V1] {
	iterator := addedTree.ContainingFirstAllNodeIterator(true)
	toAdd := iterator.Next()
	result := &opResult[E, V1]{
		key: toAdd.GetKey(),
		op:  insert,
	}
	var firstNode *BinTrieNode[E, V1]
	root := trie.absoluteRoot()
	firstAdded := toAdd.IsAdded()
	if firstAdded {
		firstNode = trie.addNode(result, root)
	} else {
		firstNode = root
	}
	lastAddedNode := firstNode
	for iterator.HasNext() {
		iterator.CacheWithLowerSubNode(lastAddedNode)
		iterator.CacheWithUpperSubNode(lastAddedNode)
		toAdd = iterator.Next()
		cachedNode := iterator.GetCached().(*BinTrieNode[E, V1])
		if toAdd.IsAdded() {
			result.key = toAdd.GetKey()
			result.existingNode = nil
			result.inserted = nil
			lastAddedNode = trie.addNode(result, cachedNode)
		} else {
			lastAddedNode = cachedNode
		}
	}
	if !firstAdded {
		firstNode = trie.GetNode(addedTree.GetKey())
	}
	return firstNode
}

func (trie *BinTrie[E, V]) AddTrie(trieNode *BinTrieNode[E, V]) *BinTrieNode[E, V] {
	if trieNode == nil {
		return nil
	}
	trie.ensureRoot(trieNode.GetKey())
	return trie.addTrie(trieNode, false)
}

func (trie *BinTrie[E, V]) Contains(key E) bool {
	return trie.absoluteRoot().Contains(key)
}

func (trie *BinTrie[E, V]) Remove(key E) bool {
	return trie.absoluteRoot().RemoveNode(key)
}

func (trie *BinTrie[E, V]) RemoveElementsContainedBy(key E) *BinTrieNode[E, V] {
	return trie.absoluteRoot().RemoveElementsContainedBy(key)
}

func (trie *BinTrie[E, V]) ElementsContainedBy(key E) *BinTrieNode[E, V] {
	return trie.absoluteRoot().ElementsContainedBy(key)
}

func (trie *BinTrie[E, V]) ElementsContaining(key E) *Path[E, V] {
	return trie.absoluteRoot().ElementsContaining(key)
}

// LongestPrefixMatch finds the key with the longest matching prefix.
func (trie *BinTrie[E, V]) LongestPrefixMatch(key E) (E, bool) {
	return trie.absoluteRoot().LongestPrefixMatch(key)
}

// LongestPrefixMatchNode finds the node with the longest matching prefix.
func (trie *BinTrie[E, V]) LongestPrefixMatchNode(key E) *BinTrieNode[E, V] {
	return trie.absoluteRoot().LongestPrefixMatchNode(key)
}

func (trie *BinTrie[E, V]) ElementContains(key E) bool {
	return trie.absoluteRoot().ElementContains(key)
}

// GetNode gets the node in the sub-trie corresponding to the given address,
// or returns nil if not such element exists.
//
// It returns any node, whether added or not,
// including any prefix block node that was not added.
func (trie *BinTrie[E, V]) GetNode(key E) *BinTrieNode[E, V] {
	return trie.absoluteRoot().GetNode(key)
}

// GetAddedNode gets trie nodes representing added elements.
//
// Use Contains to check for the existence of a given address in the trie,
// as well as GetNode to search for all nodes including those not-added but also auto-generated nodes for subnet blocks.
func (trie *BinTrie[E, V]) GetAddedNode(key E) *BinTrieNode[E, V] {
	return trie.absoluteRoot().GetAddedNode(key)
}

// Put associates the specified value with the specified key in this map.
//
// If the argument is not a single address nor prefix block, this method will panic.
// The Partition type can be used to convert the argument to single addresses and prefix blocks before calling this method.
//
// If this map previously contained a mapping for a key,
// the old value is replaced by the specified value, and false is returned along with the old value.
// If this map did not previously contain a mapping for the key, true is returned along with nil.
// The boolean return value allows you to distinguish whether the key was previously mapped to nil or not mapped at all.
func (trie *BinTrie[E, V]) Put(key E, value V) (V, bool) {
	root := trie.ensureRoot(key)
	result := &opResult[E, V]{
		key:      key,
		op:       insert,
		newValue: value,
		// new value assignment
	}
	root.matchBits(result)
	return result.existingValue, !result.exists
}

func (trie *BinTrie[E, V]) PutTrie(trieNode *BinTrieNode[E, V]) *BinTrieNode[E, V] {
	if trieNode == nil {
		return nil
	}
	trie.ensureRoot(trieNode.GetKey())
	return trie.addTrie(trieNode, true)
}

func (trie *BinTrie[E, V]) PutNode(key E, value V) *BinTrieNode[E, V] {
	root := trie.ensureRoot(key)
	result := &opResult[E, V]{
		key:      key,
		op:       insert,
		newValue: value,
		// new value assignment
	}
	root.matchBits(result)
	resultNode := result.existingNode
	if resultNode == nil {
		resultNode = result.inserted
	}
	return resultNode
}

func (trie *BinTrie[E, V]) Remap(key E, remapper func(existing V, found bool) (mapped V, mapIt bool)) *BinTrieNode[E, V] {
	return trie.remapImpl(key,
		func(existingVal V, exists bool) (V, remapAction) {
			result, mapIt := remapper(existingVal, exists)
			if mapIt {
				return result, remapValue
			}
			var v V
			//if !exists { no need for this, the remap function handles it already, it ignores removeNode when no match
			//	return v, doNothing
			//}
			return v, removeNode
		})
}

/*
func (trie *AssociativeTrie[T, V]) Remap(addr T, remapper func(existing V, found bool) (mapped V, mapIt bool)) *AssociativeTrieNode[T, V] {
	addr = mustBeBlockOrAddress(addr)
	//return toAssociativeTrieNode[T, V](trie.trieBase.trie.Remap(trieKey[T]{addr}, remapper)) // TODO reinstate once remapper is all we need, which is when bintree goes generic
	return toAssociativeTrieNode[T, V](trie.trieBase.trie.Remap(trieKey[T]{addr}, func(t tree.V) tree.V {
		var res bool
		var val V
		if t == nil {
			val, res = remapper(val, false)
		} else {
			val, res = remapper(t.(V), true)
		}
		if !res {
			return nil
		}
		return val
	}))
}
*/

func (trie *BinTrie[E, V]) RemapIfAbsent(key E, supplier func() V) *BinTrieNode[E, V] {
	return trie.remapImpl(key,
		func(existingVal V, exists bool) (V, remapAction) {
			if !exists {
				return supplier(), remapValue
			}
			var v V
			return v, doNothing
		})
}

/*
func (trie *AssociativeTrie[T, V]) RemapIfAbsent(addr T, supplier func() (V, bool)) *AssociativeTrieNode[T, V] {
	addr = mustBeBlockOrAddress(addr)
	//return toAssociativeTrieNode[T, V](trie.trieBase.trie.RemapIfAbsent(trieKey[T]{addr}, supplier)) // TODO reinstate once bintree uses generics too,  remapper is all we need
	return toAssociativeTrieNode[T, V](trie.trieBase.trie.RemapIfAbsent(trieKey[T]{addr}, func() tree.V {
		val, res := supplier()
		if !res {
			return nil
		}
		return val
	}, false))
}
*/
/*
// RemapIfAbsent remaps node values in the trie, but only for nodes that do not exist or are mapped to nil.
//
// This will look up the node corresponding to the given key.
// If the node is not found or mapped to nil, this will call the remapping function.
//
// If the remapping function returns a non-nil value, then it will either set the existing node to have that value,
// or if there was no matched node, it will create a new node with that value.
// If the remapping function returns nil, then it will do the same if insertNil is true, otherwise it will do nothing.
//
// The method will return the node involved, which is either the matched node, or the newly created node,
// or nil if there was no matched node nor newly created node.
//
// If the remapping function modifies the trie during its computation,
// and the returned value specifies changes to be made,
// then the trie will not be changed and ConcurrentModificationException will be thrown instead.
//
// If the argument is not a single address nor prefix block, this method will panic.
// The Partition type can be used to convert the argument to single addresses and prefix blocks before calling this method.
//
// insertNull indicates whether nil values returned from remapper should be inserted into the map, or whether nil values indicate no remapping
func (trie *AssociativeAddressTrie) RemapIfAbsent(addr *Address, supplier func() NodeValue, insertNil bool) *AssociativeAddressTrieNode {
	return trie.remapIfAbsent(addr, supplier, insertNil)
}
*/

func (trie *BinTrie[E, V]) remapImpl(key E, remapper func(val V, exists bool) (V, remapAction)) *BinTrieNode[E, V] {
	root := trie.ensureRoot(key)
	result := &opResult[E, V]{
		key:      key,
		op:       remap,
		remapper: remapper,
	}
	root.matchBits(result)
	resultNode := result.existingNode
	if resultNode == nil {
		resultNode = result.inserted
	}
	return resultNode
}

func (trie *BinTrie[E, V]) Get(key E) (V, bool) {
	return trie.absoluteRoot().Get(key)
}

// NodeIterator returns an iterator that iterates through the added nodes of the trie in forward or reverse tree order.
func (trie *BinTrie[E, V]) NodeIterator(forward bool) TrieNodeIteratorRem[E, V] {
	return trie.absoluteRoot().NodeIterator(forward)
}

// AllNodeIterator returns an iterator that iterates through all the nodes of the trie in forward or reverse tree order.
func (trie *BinTrie[E, V]) AllNodeIterator(forward bool) TrieNodeIteratorRem[E, V] {
	return trie.absoluteRoot().AllNodeIterator(forward)
}

// BlockSizeNodeIterator returns an iterator that iterates the added nodes in the trie, ordered by keys from largest prefix blocks to smallest, and then to individual addresses.
//
// If lowerSubNodeFirst is true, for blocks of equal size the lower is first, otherwise the reverse order
func (trie *BinTrie[E, V]) BlockSizeNodeIterator(lowerSubNodeFirst bool) TrieNodeIteratorRem[E, V] {
	return trie.absoluteRoot().BlockSizeNodeIterator(lowerSubNodeFirst)
}

// BlockSizeAllNodeIterator returns an iterator that iterates all nodes in the trie, ordered by keys from largest prefix blocks to smallest, and then to individual addresses.
//
// If lowerSubNodeFirst is true, for blocks of equal size the lower is first, otherwise the reverse order
func (trie *BinTrie[E, V]) BlockSizeAllNodeIterator(lowerSubNodeFirst bool) TrieNodeIteratorRem[E, V] {
	return trie.absoluteRoot().BlockSizeAllNodeIterator(lowerSubNodeFirst)
}

// BlockSizeCachingAllNodeIterator returns an iterator that iterates all nodes, ordered by keys from largest prefix blocks to smallest, and then to individual addresses.
func (trie *BinTrie[E, V]) BlockSizeCachingAllNodeIterator() CachingTrieNodeIterator[E, V] {
	return trie.absoluteRoot().BlockSizeCachingAllNodeIterator()
}

func (trie *BinTrie[E, V]) ContainingFirstIterator(forwardSubNodeOrder bool) CachingTrieNodeIterator[E, V] {
	return trie.absoluteRoot().ContainingFirstIterator(forwardSubNodeOrder)
}

func (trie *BinTrie[E, V]) ContainingFirstAllNodeIterator(forwardSubNodeOrder bool) CachingTrieNodeIterator[E, V] {
	return trie.absoluteRoot().ContainingFirstAllNodeIterator(forwardSubNodeOrder)
}

func (trie *BinTrie[E, V]) ContainedFirstIterator(forwardSubNodeOrder bool) TrieNodeIteratorRem[E, V] {
	return trie.absoluteRoot().ContainedFirstIterator(forwardSubNodeOrder)
}

func (trie *BinTrie[E, V]) ContainedFirstAllNodeIterator(forwardSubNodeOrder bool) TrieNodeIterator[E, V] {
	return trie.absoluteRoot().ContainedFirstAllNodeIterator(forwardSubNodeOrder)
}

func (trie *BinTrie[E, V]) FirstNode() *BinTrieNode[E, V] {
	return trie.absoluteRoot().FirstNode()
}

func (trie *BinTrie[E, V]) FirstAddedNode() *BinTrieNode[E, V] {
	return trie.absoluteRoot().FirstAddedNode()
}

func (trie *BinTrie[E, V]) LastNode() *BinTrieNode[E, V] {
	return trie.absoluteRoot().LastNode()
}

func (trie *BinTrie[E, V]) LastAddedNode() *BinTrieNode[E, V] {
	return trie.absoluteRoot().LastAddedNode()
}

func (trie *BinTrie[E, V]) LowerAddedNode(key E) *BinTrieNode[E, V] {
	return trie.absoluteRoot().LowerAddedNode(key)
}

func (trie *BinTrie[E, V]) FloorAddedNode(key E) *BinTrieNode[E, V] {
	return trie.absoluteRoot().FloorAddedNode(key)
}

func (trie *BinTrie[E, V]) HigherAddedNode(key E) *BinTrieNode[E, V] {
	return trie.absoluteRoot().HigherAddedNode(key)
}

func (trie *BinTrie[E, V]) CeilingAddedNode(key E) *BinTrieNode[E, V] {
	return trie.absoluteRoot().CeilingAddedNode(key)
}

func (trie *BinTrie[E, V]) Clone() *BinTrie[E, V] {
	if trie == nil {
		return nil
	}
	return &BinTrie[E, V]{binTree[E, V]{root: trie.absoluteRoot().CloneTree().toBinTreeNode()}}
}

// DeepEqual returns whether the given argument is a trie with a set of nodes with the same keys as in this trie according to the Compare method,
// and the same values according to the reflect.DeepEqual method
func (trie *BinTrie[E, V]) DeepEqual(other *BinTrie[E, V]) bool {
	return trie.absoluteRoot().TreeDeepEqual(other.absoluteRoot())
}

// Equal returns whether the given argument is a trie with a set of nodes with the same keys as in this trie according to the Compare method
func (trie *BinTrie[E, V]) Equal(other *BinTrie[E, V]) bool {
	return trie.absoluteRoot().TreeEqual(other.absoluteRoot())
}

// For some reason Format must be here and not in addressTrieNode for nil node.
// It panics in fmt code either way, but if in here then it is handled by a recover() call in fmt properly.
// Seems to be a problem only in the debugger.

// Format implements the fmt.Formatter interface
func (trie BinTrie[E, V]) Format(state fmt.State, verb rune) {
	trie.format(state, verb)
}

// NewBinTrie creates a new trie with root key.ToPrefixBlockLen(0).
// If the key argument is not Equal to its zero-length prefix block, then the key will be added as well.
func NewBinTrie[E TrieKey[E], V any](key E) BinTrie[E, V] {
	trie := BinTrie[E, V]{binTree[E, V]{}}
	root := key.ToPrefixBlockLen(0)
	trie.setRoot(root)
	if key.Compare(root) != 0 {
		trie.Add(key)
	}
	return trie
}

func TreesString[E TrieKey[E], V any](withNonAddedKeys bool, tries ...*BinTrie[E, V]) string {
	binTrees := make([]*binTree[E, V], 0, len(tries))
	for _, trie := range tries {
		binTrees = append(binTrees, tobinTree(trie))
	}
	return treesString(withNonAddedKeys, binTrees...)
}

func tobinTree[E TrieKey[E], V any](trie *BinTrie[E, V]) *binTree[E, V] {
	return (*binTree[E, V])(unsafe.Pointer(trie))
}
