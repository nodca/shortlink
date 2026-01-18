package le

type ListNode struct {
	Val  int
	Next *ListNode
}

func reverseKGroup(head *ListNode, k int) *ListNode {
	cur := head
	len := 0
	for cur != nil {
		len++
		cur = cur.Next
	}

	dummy := &ListNode{Next: head}
	left, mid := dummy, head
	p0 := dummy

	times := len / k
	for range times {
		for range k {
			right := mid.Next
			mid.Next = left
			left = mid
			mid = right
		}
		nxt := p0.Next
		p0.Next.Next = mid
		p0.Next = left
		p0 = nxt
	}
	return dummy.Next
}
