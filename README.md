# ipapk
ipa or apk parser written in golang, aims to extract app information

[![Build Status](https://travis-ci.org/qiqizjl/ipapk.svg?branch=master)](https://travis-ci.org/qiqizjl/ipapk)

## INSTALL
	$ go get github.com/qiqizjl/ipapk
  
## USAGE
```go
package main

import (
	"fmt"
	"github.com/qiqizjl/ipapk"
)

func main() {
	apk, _ := ipapk.NewAppParser("test.apk")
	fmt.Println(apk)
}
```
