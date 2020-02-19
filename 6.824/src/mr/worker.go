package mr

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"sync"
	"time"
)
import "log"
import "net/rpc"
import "hash/fnv"

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

type WorkerServer struct {
	sync.Mutex

	WorkerId     int
	MapFunc      func(string, string) []KeyValue
	ReduceFunc   func(string, []string) string
	FailureCount int
}

//
// use ihash(key) % NReduce to choose the reduce
// Task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}


//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// Your worker implementation here.

	worker := WorkerServer{}
	worker.MapFunc = mapf
	worker.ReduceFunc = reducef
	rArgs := RegisterRequest{Occupy:0}
	rReply := RegisterReply{}
	call("Master.Register", &rArgs, &rReply)
	worker.WorkerId = rReply.WorkId
	fmt.Printf("worker %v init\n", worker.WorkerId)

	for true {
		args := TaskRequest{}
		args.WorkerId = worker.WorkerId
		args.RequestType = 0 // Request for Task
		reply := TaskReply{}
		done := call("Master.Schedule", &args, &reply)
		worker.Lock()
		if done == false {
			if worker.FailureCount >= 1 {
				worker.Unlock()
				fmt.Printf("worker %v finish\n", worker.WorkerId)
				break
			} else {
				worker.FailureCount++
			}
		}
		worker.Unlock()
		switch reply.Task {
		// Map Task
		case 0:
			var intermediate []*os.File
			for i := 0; i < reply.NReduce; i++ {
				filename := "tmp-"+strconv.Itoa(reply.TaskNum)+"-"+strconv.Itoa(i)
				os.Remove(filename)
				file, _ := os.Create(filename)
				intermediate = append(intermediate, file)
			}
			for _, filename := range reply.File {
				file, err := os.Open(filename)
				if err != nil {
					log.Fatalf("cannot open %v", filename)
				}
				content, err := ioutil.ReadAll(file)
				if err != nil {
					log.Fatalf("cannot read %v", filename)
				}
				err = file.Close()
				if err != nil {
					log.Fatal(err)
				}
				kva := mapf(filename, string(content))
				for _, keyValue := range kva {
					enc := json.NewEncoder(intermediate[ihash(keyValue.Key)%reply.NReduce])
					err = enc.Encode(keyValue)
					if err != nil {
						panic(err)
					}
				}
			}
			for i, file := range intermediate {
				filename := "mr-"+
					strconv.Itoa(reply.TaskNum)+"-"+strconv.Itoa(i)
				os.Remove(filename)
				file.Close()
				os.Rename(file.Name(), filename)
			}
			args := TaskRequest{}
			args.WorkerId = worker.WorkerId
			args.RequestType = 1
			fmt.Printf("worker %v finish Task %v\n", worker.WorkerId,
				reply.TaskNum)
			reply := TaskReply{}
			call("Master.Schedule", &args, &reply)
		// Reduce Task
		case 1:
			kva := make(map[string][]string)
			for _, filename := range reply.File {
				file, err := os.Open(filename)
				// Some task may crash, but taskNum still add.
				if err != nil {
					continue
				}
				dec := json.NewDecoder(file)
				for {
					var kv KeyValue
					if err := dec.Decode(&kv); err != nil {
						break
					}
					kva[kv.Key] = append(kva[kv.Key], kv.Value)
				}
				file.Close()
			}
			filename := "tmp-out-" + strconv.Itoa(reply.TaskNum)
			os.Remove(filename)
			file, _ := os.Create(filename)
			for key, value := range kva {
				fmt.Fprintf(file, "%v %v\n", key, worker.ReduceFunc(key, value))
			}
			filename = "mr-out-" + strconv.Itoa(reply.TaskNum)
			os.Remove(filename)
			file.Close()
			os.Rename("tmp-out-" + strconv.Itoa(reply.TaskNum), filename)
			args := TaskRequest{}
			args.WorkerId = worker.WorkerId
			args.RequestType = 1
			fmt.Printf("worker %v finish Task %v\n", worker.WorkerId,
				reply.TaskNum)
			reply := TaskReply{}
			call("Master.Schedule", &args, &reply)
		// idle
		case 2:
			//fmt.Printf("No available task for worker %v\n", worker.WorkerId)
			time.Sleep(2*time.Second)
		}
	}


}


//
// example function to show how to make an RPC call to the master.
//
// the RPC argument and reply types are defined in rpc.go.
//

//func CallExample() {
//
//	// declare an argument structure.
//	args := ExampleArgs{}
//
//	// fill in the argument(s).
//	args.X = 99
//
//	// declare a reply structure.
//	reply := ExampleReply{}
//
//	// send the RPC request, wait for the reply.
//	call("Master.Example", &args, &reply)
//
//	// reply.Y should be 100.
//	fmt.Printf("reply.Y %v\n", reply.Y)
//}

//
// send an RPC request to the master, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := masterSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
