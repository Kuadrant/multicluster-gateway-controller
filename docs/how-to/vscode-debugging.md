# Debugging in VS code

## Introduction
The following document will show how to setup debugging for multi gateway controller.

There is an included VSCode `launch.json`.

## Starting the controller

Instead of starting the Gateway Controller via something like:

```bash
make build-controller install run-controller
```

You can now simply hit `F5` in VSCode. The controller will launch with the following config:

```
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "./cmd/controller/main.go",
      "args": [
        "--metrics-bind-address=:8080",
        "--health-probe-bind-address=:8081"
      ]
    }
  ]
}
```

### Running Debugger
![VSCode Debugger 1](images/vscode-1.png)


### Debugging Tests
![VSCode Debugger 2](images/vscode-2.png)

