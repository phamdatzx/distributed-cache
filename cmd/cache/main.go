package main

import (
	"fmt"
)

func sum(a []int, c chan int){
    sum := 0
    for _, v := range a {
        sum += v
    }
    c <- sum
}

func main() {
	a := []int{1,2,3}
    b := []int{4,5,6}

	c := make(chan int)
	go sum(a, c)
	go sum(b, c)
	x, y := <-c, <-c // receive from c

	fmt.Println(x, y, x+y)
}