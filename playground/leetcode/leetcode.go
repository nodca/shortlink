package leetcode

import (
	"container/list"
	"math/rand/v2"
)

func lengthOfLongestSubstring(s string) int {
	ans := 0
	window := [128]bool{}
	left := 0
	for right, ch := range s {
		for window[ch] {
			window[s[left]] = false
			left++
		}
		window[ch] = true
		ans = max(ans, right-left+1)
	}
	return ans
}

type ListNode struct {
	Next *ListNode
	Val  int
}

func reverseList(head *ListNode) *ListNode {
	var left, mid *ListNode = nil, head
	for mid != nil {
		right := mid.Next
		mid.Next = left
		left = mid
		mid = right
	}
	return left
}

/***************LRU******************************/
type entry struct {
	key, value int
}
type LRUCache struct {
	Capacity  int
	cacheList *list.List
	KeyToNOde map[int]*list.Element
}

func Constructor(capacity int) LRUCache {
	return LRUCache{
		Capacity:  capacity,
		cacheList: list.New(),
		KeyToNOde: make(map[int]*list.Element),
	}
}

func (this *LRUCache) Get(key int) int {
	node := this.KeyToNOde[key]
	if node == nil {
		return -1
	}
	this.cacheList.MoveToFront(node)
	return node.Value.(entry).value
}

func (this *LRUCache) Put(key int, value int) {
	if node := this.KeyToNOde[key]; node != nil {
		this.cacheList.MoveToFront(node)
		node.Value = entry{key, value}
		return
	}
	this.KeyToNOde[key] = this.cacheList.PushFront(entry{key: key, value: value})
	if len(this.KeyToNOde) > this.Capacity {
		bnode := this.cacheList.Remove(this.cacheList.Back())
		delete(this.KeyToNOde, bnode.(entry).key)
	}
}

/*********************第K大元素（快速选择）**********************************/
func partition(nums []int, left int, right int) int {
	i := left + rand.IntN(right-left+1)
	pivot := nums[i]
	nums[left], nums[i] = nums[i], nums[left]
	i = left + 1
	j := right
	for {
		for i <= j && nums[i] < pivot {
			i++
		}
		for i <= j && nums[j] > pivot {
			j--
		}
		if i >= j {
			break
		}
		nums[i], nums[j] = nums[j], nums[i]
		i++
		j--
	}
	nums[left], nums[j] = nums[j], nums[left]
	return j
}

func findKthLargest(nums []int, k int) int {
	n := len(nums)
	left, right := 0, n-1
	for {
		index := partition(nums, left, right)
		if index == (n - k) {
			return nums[index]
		} else if index < (n - k) {
			left = index + 1
		} else {
			right = index - 1
		}
	}
}
