package containermanager

var strTestCompiler string = `package main

import (
	"fmt"
	"time"
)

func wait(done chan struct{}) {
	time.Sleep(time.Second)
	done <- struct{}{}
}

func main() {
	done := make(chan struct{})
	fmt.Printf("init %v\n", time.Now())
	go wait(done)
	<-done
	fmt.Printf("done 1 %v\n", time.Now())
	go wait(done)
	<-done
	fmt.Printf("done 2 %v\n", time.Now())
}`

var strTestSequentialRun string = `package main

import (
	"fmt"

	"github.com/bhmj/jsonslice"
)

func main() {
	var data = []byte(` + "`" + `
	{ "sku": [ 
	    { "id": 1, "name": "Bicycle", "price": 160, "extras": [ "flashlight", "pump" ] },
	    { "id": 2, "name": "Scooter", "price": 280, "extras": [ "helmet", "gloves", "spare wheel" ] }
	  ]
	} ` + "`" + `)

	a, _ := jsonslice.Get(data, "$.sku[0].price")
	b, _ := jsonslice.Get(data, "$.sku[1].extras.count()")
	c, _ := jsonslice.Get(data, "$.sku[?(@.price > 200)].name")
	d, _ := jsonslice.Get(data, "$.sku[?(@.extras.count() < 3)].name")

	fmt.Println(string(a)) // 160
	fmt.Println(string(b)) // 3
	fmt.Println(string(c)) // ["Scooter"]
	fmt.Println(string(d)) // ["Bicycle"]
}`

var strTestSpawn string = `package main

import (
	"os/exec"
)

func main() {
	cmd := exec.Command("sleep", "60") // spawn process!
	cmd.Start()
}
`

var strTestRW string = `package main

import (
	"os/exec"
)

func main() {
	cmd := exec.Command("sleep", "60")
	cmd.Start()
}
`
