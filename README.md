# GO Workflow Demo for Dapr

This is a go version human approval workflow inspired from [python-sdk](https://github.com/dapr/python-sdk/blob/main/examples/workflow/human_approval.py).

* Run daprd by:
```
dapr init
dapr run --app-id wfexample --dapr-grpc-port 50001      
```

* Run current workflow

```
go run demo.go
```