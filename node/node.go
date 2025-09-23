package node

type Node struct {
	Name            string
	Ip              string
	Cores           int
	Memory          int
	MemoryAllocated int
	Disk            int
	DiskAllocated   int
	Role            string
	TaskCount       int
}

func NewNode(name string, address string, role string) *Node {
	return &Node{
		Name: name,
		Ip:   address,
		Role: role,
	}
}
