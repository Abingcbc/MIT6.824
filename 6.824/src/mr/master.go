package mr

import (
	"fmt"
	"log"
	"strconv"
	"time"
)
import "net"
import "os"
import "net/rpc"
import "net/http"
import "sync"


type Master struct {
	// Your definitions here.
	sync.Mutex

	NReduce     int
	Files       []string
	MTask       int
	RTask       []int
	Worker2task map[int]Task
	phase       int
	workerNum   int
	ValidTask  []int
}

type Task struct {
	TaskNum int
	File    []string
}

// Your code here -- RPC handlers for the worker to call.

func (m *Master) Schedule(args *TaskRequest, reply *TaskReply) error {
	m.Lock()
	switch m.phase {
	// Map phase
	case 0:
		switch args.RequestType {
		case 0:
			if len(m.Files) > 0 {
				// As I don't know the number of map workers, one File one time.
				reply.File = append(reply.File, m.Files[len(m.Files)-1])
				m.Files = m.Files[:len(m.Files)-1]
				reply.Task = 0 // Map Task
				reply.TaskNum = m.MTask
				fmt.Printf("Assign Map task %v to worker %v\n", m.MTask, args.WorkerId)
				reply.NReduce = m.NReduce
				m.Worker2task[args.WorkerId] = Task{TaskNum: m.MTask, File: reply.File}
				m.MTask++
				go func() {
					time.Sleep(10*time.Second)
					m.Lock()
					if value, ok := m.Worker2task[args.WorkerId]; ok {
						if value.TaskNum == reply.TaskNum {
							fmt.Printf("Worker %v crashed\n", args.WorkerId)
							for _, file := range value.File {
								m.Files = append(m.Files, file)
							}
							delete(m.Worker2task, args.WorkerId)
						}
					}
					m.Unlock()
				}()
			} else if len(m.Worker2task) > 0 {
				fmt.Printf("Waiting for Map tasks...\n")
				reply.Task = 2 // idle
			}
		case 1:
			m.ValidTask = append(m.ValidTask, m.Worker2task[args.WorkerId].TaskNum)
			delete(m.Worker2task, args.WorkerId)
			if len(m.Files) == 0 && len(m.Worker2task) == 0 {
				fmt.Printf("Map finish, start reduce\n")
				m.phase = 1
			}
		}
	// Reduce phase
	case 1:
		switch args.RequestType {
		case 0:
			if len(m.RTask) != 0 {
				reply.Task = 1
				reply.TaskNum = m.RTask[len(m.RTask)-1]
				fmt.Printf("Assign Reduce task %v to worker %v\n", reply.TaskNum, args.WorkerId)
				var reduceFiles []string
				for _, taskNum := range m.ValidTask{
					reduceFiles = append(reduceFiles, "mr-"+strconv.Itoa(taskNum)+"-"+
						strconv.Itoa(m.RTask[len(m.RTask)-1]))
				}
				reply.File = reduceFiles
				m.Worker2task[args.WorkerId] = Task{
					TaskNum: reply.TaskNum,
					File:    reply.File,
				}
				m.RTask = m.RTask[:len(m.RTask)-1]
				go func() {
					time.Sleep(10*time.Second)
					m.Lock()
					if value, ok := m.Worker2task[args.WorkerId]; ok {
						if value.TaskNum == reply.TaskNum {
							fmt.Printf("Worker %v crashed\n", args.WorkerId)
							m.RTask = append(m.RTask, value.TaskNum)
							delete(m.Worker2task, args.WorkerId)
						}
					}
					m.Unlock()
				}()
			} else {
				fmt.Printf("Waiting for Reduce tasks...\n")
				reply.Task = 2 //idle
			}
		// Task finish
		case 1:
			delete(m.Worker2task, args.WorkerId)
			if len(m.RTask) == 0 && len(m.Worker2task) == 0 {
				fmt.Printf("All tasks finished\n")
				m.phase = 2
			}
		}
	// Finish
	case 2:
		reply.Task = 2
	}
	m.Unlock()
	return nil
}

func (m *Master) Register(args *RegisterRequest, reply *RegisterReply) error {
	m.Lock()
	reply.WorkId = m.workerNum
	m.workerNum++
	m.Unlock()
	return nil
}

//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
//func (m *Master) Example(args *ExampleArgs, reply *ExampleReply) error {
//	reply.Y = args.X + 1
//	return nil
//}

//
// start a thread that listens for RPCs from worker.go
//
func (m *Master) server() {
	rpc.Register(m)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := masterSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrmaster.go calls Done() periodically to find out
// if the entire job has finished.
//
func (m *Master) Done() bool {
	ret := false

	// Your code here.
	m.Lock()
	if m.phase == 2 {
		ret = true
	}
	m.Unlock()
	return ret
}

//
// create a Master.
// main/mrmaster.go calls this function.
// NReduce is the number of reduce tasks to use.
//
func MakeMaster(files []string, nReduce int) *Master {
	m := Master{}

	// Your code here.
	m.Lock()
	m.Files = files
	m.NReduce = nReduce
	m.Worker2task = map[int]Task{}
	m.phase = 0
	m.workerNum = 0
	for i := 0; i < nReduce; i++ {
		m.RTask = append(m.RTask, i)
	}
	m.Unlock()
	m.server()
	return &m
}
