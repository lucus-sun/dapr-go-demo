package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/microsoft/durabletask-go/api"
	"github.com/microsoft/durabletask-go/backend"
	"github.com/microsoft/durabletask-go/client"
	"github.com/microsoft/durabletask-go/task"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Order struct {
	Cost     float64
	Product  string
	Quantity int64
}

type Approval struct {
	Approver string
}

func purchase_order_workflow(ctx *task.OrchestrationContext) (any, error) {
	fmt.Println("Starting workflow...")
	var order Order
	err := ctx.GetInput(&order)
	if err != nil {
		return nil, err
	}
	if order.Cost < 1000 {
		return "Auto-approved", nil
	}
	var res string
	err = ctx.CallActivity("send_approval_request").Await(&res)
	if err != nil {
		return nil, err
	}
	fmt.Println("called activity:", res)
	approval := Approval{}
	err = ctx.WaitForSingleEvent("approval_received", time.Duration(10)*time.Second).Await(&approval)
	if errors.As(err, task.ErrTaskCanceled) {
		return "Waiting Approval Timeout", nil
	}
	if err != nil {
		return nil, err
	}

	return fmt.Sprintf("Approved by %s", approval.Approver), nil
}

func main() {
	r := task.NewTaskRegistry()
	r.AddOrchestratorN("purchase_order_workflow", purchase_order_workflow)
	r.AddActivityN("send_approval_request", func(ctx task.ActivityContext) (any, error) {
		fmt.Println("Sending approval request...")
		return nil, nil
	})

	conn, err := grpc.Dial("localhost:50001", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()
	grpcClient := client.NewTaskHubGrpcClient(conn, backend.DefaultLogger())
	err = grpcClient.StartWorkItemListener(context.TODO(), r)
	if err != nil {
		log.Fatal("failed to start work item listener: %v", err)
	}

	id, err := grpcClient.ScheduleNewOrchestration(context.TODO(), "purchase_order_workflow", api.WithInput(Order{
		Cost: 1001,
	}))
	if err != nil {
		log.Fatal("failed to schedule new orchestration: %v", err)
	}

	go func() {
		time.Sleep(3 * time.Second)
		fmt.Println("press any button to continue...")
		var input string
		fmt.Scanln(&input)
		fmt.Println("got approved event...")
		suberr := grpcClient.RaiseEvent(context.TODO(), id, "approval_received", api.WithEventPayload(&Approval{Approver: "Myself"}))
		if suberr != nil {
			log.Fatal("failed to raise event: %v", suberr)
		}
	}()

	timeoutCtx, cancelTimeout := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancelTimeout()
	metadata, err := grpcClient.WaitForOrchestrationCompletion(timeoutCtx, id, api.WithFetchPayloads(true))
	if err != nil {
		log.Fatalf("failed to wait for orchestration completion: %v", err)
	}
	fmt.Println("Workflow finished:", metadata.SerializedOutput)
}
