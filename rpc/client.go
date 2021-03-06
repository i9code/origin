package rpc

import (
	"fmt"
	"github.com/duanhf2012/origin/log"
	"github.com/duanhf2012/origin/network"
	"math"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Client struct {
	blocalhost bool
	network.TCPClient
	conn *network.TCPConn

	pendingLock sync.RWMutex
	startSeq uint64
	pending map[uint64]*Call
}

func (slf *Client) NewClientAgent(conn *network.TCPConn) network.Agent {
	slf.conn = conn
	slf.pending = map[uint64]*Call{}
	return slf
}

func (slf *Client) Connect(addr string) error {
	slf.Addr = addr
	if strings.Index(addr,"localhost") == 0 {
		slf.blocalhost = true
		return nil
	}
	slf.ConnNum = 1
	slf.ConnectInterval = time.Second*2
	slf.PendingWriteNum = 10000
	slf.AutoReconnect = true
	slf.LenMsgLen =2
	slf.MinMsgLen = 2
	slf.MaxMsgLen = math.MaxUint16
	slf.NewAgent =slf.NewClientAgent
	slf.LittleEndian = LittleEndian

	slf.pendingLock.Lock()
	for _,v := range slf.pending {
		v.Err = fmt.Errorf("node is disconnect.")
		v.done <- v
	}
	slf.pending = map[uint64]*Call{}
	slf.pendingLock.Unlock()
	slf.Start()
	return nil
}

func (slf *Client) AsycGo(rpcHandler IRpcHandler,serviceMethod string,callback reflect.Value, args interface{},replyParam interface{}) error {
	call := new(Call)
	call.Reply = replyParam
	call.callback = &callback
	call.rpcHandler = rpcHandler
	call.ServiceMethod = serviceMethod

	InParam,herr := processor.Marshal(args)
	if herr != nil {
		return herr
	}

	request := &RpcRequest{}
	call.Arg = args
	slf.pendingLock.Lock()
	slf.startSeq += 1
	call.Seq = slf.startSeq
	//request.Seq = slf.startSeq
	//request.NoReply = false
	//request.ServiceMethod = serviceMethod
	request.RpcRequestData = processor.MakeRpcRequest(slf.startSeq,serviceMethod,false,InParam)
	slf.pending[call.Seq] = call
	slf.pendingLock.Unlock()



	bytes,err := processor.Marshal(request.RpcRequestData)
	if err != nil {
		return err
	}
	if slf.conn == nil {
		return fmt.Errorf("Rpc server is disconnect,call %s is fail!",serviceMethod)
	}
	err = slf.conn.WriteMsg(bytes)
	if err != nil {
		call.Err = err
	}

	return call.Err
}

func (slf *Client) Go(noReply bool,serviceMethod string, args interface{},reply interface{}) *Call {
	call := new(Call)
	call.done = make(chan *Call,1)
	call.Reply = reply
	call.ServiceMethod = serviceMethod

	InParam,err := processor.Marshal(args)
	if err != nil {
		call.Err = err
		return call
	}

	request := &RpcRequest{}
	//request.NoReply = noReply
	call.Arg = args
	slf.pendingLock.Lock()
	slf.startSeq += 1
	call.Seq = slf.startSeq
	//request.Seq = slf.startSeq
	//	request.ServiceMethod = serviceMethod
	request.RpcRequestData = processor.MakeRpcRequest(slf.startSeq,serviceMethod,noReply,InParam)
	slf.pending[call.Seq] = call
	slf.pendingLock.Unlock()


	bytes,err := processor.Marshal(request.RpcRequestData)
	if err != nil {
		call.Err = err
		return call
	}

	if slf.conn == nil {
		call.Err = fmt.Errorf("call %s is fail,rpc client is disconnect.",serviceMethod)
		return call
	}
	
	err = slf.conn.WriteMsg(bytes)
	if err != nil {
		call.Err = err
	}

	return call
}

type RequestHandler func(Returns interface{},Err *RpcError)

type RpcRequest struct {
	RpcRequestData IRpcRequestData

	//packhead
	/*Seq uint64             // sequence number chosen by client
	ServiceMethod string   // format: "Service.Method"
	NoReply bool           //??????????????????
	//packbody
	InParam []byte
*/
	//other data
	localReply interface{}
	localParam interface{} //???????????????????????????
	requestHandle RequestHandler
	callback *reflect.Value
}

type RpcResponse struct {
	RpcResponeData IRpcResponseData
	/*
	//head
	Seq           uint64   // sequence number chosen by client
	Err *RpcError

	//returns
	Reply []byte*/
}


func (slf *Client) Run(){
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			l := runtime.Stack(buf, false)
			err := fmt.Errorf("%v: %s\n", r, buf[:l])
			log.Error("core dump info:%+v",err)
		}
	}()

	for {
		bytes,err := slf.conn.ReadMsg()
		if err != nil {
			log.Error("rpcClient %s ReadMsg error:%+v",slf.Addr,err)
			return
		}
		//1.??????head
		respone := &RpcResponse{}
		respone.RpcResponeData =processor.MakeRpcResponse(0,nil,nil)
		err = processor.Unmarshal(bytes,respone.RpcResponeData)
		if err != nil {
			log.Error("rpcClient Unmarshal head error,error:%+v",err)
			continue
		}

		slf.pendingLock.Lock()
		v,ok := slf.pending[respone.RpcResponeData.GetSeq()]
		if ok == false {
			log.Error("rpcClient cannot find seq %d in pending",respone.RpcResponeData.GetSeq())
			slf.pendingLock.Unlock()
		}else  {
			delete(slf.pending,respone.RpcResponeData.GetSeq())
			slf.pendingLock.Unlock()
			v.Err = nil

			if len(respone.RpcResponeData.GetReply()) >0 {
				err = processor.Unmarshal(respone.RpcResponeData.GetReply(),v.Reply)
				if err != nil {
					log.Error("rpcClient Unmarshal body error,error:%+v",err)
					v.Err = err
				}
			}

			if respone.RpcResponeData.GetErr() != nil {
				v.Err= respone.RpcResponeData.GetErr()
			}

			if v.callback!=nil && v.callback.IsValid() {
				 v.rpcHandler.(*RpcHandler).callResponeCallBack<-v
			}else{
				v.done <- v
			}
		}
	}
}

func (slf *Client) OnClose(){
}

func (slf *Client) IsConnected() bool {
	return slf.conn!=nil && slf.conn.IsConnected()==true
}