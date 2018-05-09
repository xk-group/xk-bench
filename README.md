# xk-bench

## Install Go

env setup:
```
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
```

## Download & Build

```
go get github.com/xk-group/xk-bench
go install github.com/xk-group/xk-bench
```
If built successfully, the binary should loacted at `$GOPATH/bin`,

```
export PATH=$PATH:$GOPATH/bin
```

## Usage

```
xk-bench -n 10000 -C 1000 -i setlimit.urls -t 5
```
This command will send 10000 requests from setlimit.urls randomly, will 1000 concurrency, each request' timeout is set to 5 seconds.

When the parameter of `-i` option is set as `-`, or without `-i` option, the program will read from stdin.
