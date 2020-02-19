package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import "os"
import "strconv"

//
// example to show how to declare the arguments
// and reply for an RPC.
//

type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

// Add your RPC definitions here.

type TaskRequest struct {
	WorkerId    int
	RequestType int

}

type TaskReply struct {
	WorkerId int
	Task    int
	TaskNum int
	File    []string // support multi-input
	NReduce int      // If rReduce is a const,
				// maybe this can be set when the worker inits?
}

type RegisterRequest struct {
	Occupy int
}

type RegisterReply struct {
	WorkId int
}


// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the master.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func masterSock() string {
	s := "/var/tmp/824-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
