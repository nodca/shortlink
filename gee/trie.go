package gee

import "strings"

type node struct {
	pattern  string  //待匹配路由，如/p/:lang
	part     string  //待匹配路由的一部分,例如:lang
	children []*node //子节点
	isWild   bool    //是否是精确匹配,part含有:或*时为true
}

/*
当我们匹配 /p/go/doc/这个路由时，
第一层节点，p精准匹配到了p，
第二层节点，go模糊匹配到:lang，
那么将会把lang这个参数赋值为go，
继续下一层匹配。
*/
// 第一个匹配成功的节点，用于插入
func (n *node) matchChild(part string) *node {
	for _, child := range n.children {
		if child.part == part {
			return child
		}
	}
	return nil
}

// 所有匹配成功的节点，用于查找
func (n *node) matchChildren(part string) []*node {
	nodes := make([]*node, 0)
	for _, child := range n.children {
		if child.part == part {
			nodes = append(nodes, child)
		}
	}
	for _, child := range n.children {
		if child.isWild {
			nodes = append(nodes, child)
		}
	}
	return nodes
}
func (n *node) insert(pattern string, parts []string, height int) {
	if len(parts) == height {
		//最后在叶子节点，赋值完整的pattern作为路由终点
		n.pattern = pattern
		return
	}
	part := parts[height]
	child := n.matchChild(part)
	if child == nil {
		child = &node{
			part:   part,
			isWild: part[0] == ':' || part[0] == '*',
		}
		n.children = append(n.children, child)
	}
	//这里是用child，下一个递归的n会自动变成子节点。
	child.insert(pattern, parts, height+1)
}

func (n *node) search(parts []string, height int) *node {
	if len(parts) == height || strings.HasPrefix(n.part, "*") {
		if n.pattern == "" {
			return nil
		}
		return n
	}

	part := parts[height]
	children := n.matchChildren(part)

	for _, child := range children {
		result := child.search(parts, height+1)
		if result != nil {
			return result
		}
	}
	return nil
}
