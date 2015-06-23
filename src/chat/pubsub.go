/*
This package provides a portable intaface to pubsub model.
PubSub can publish/subscribe/unsubscribe messages for all.
To subscribe:

	ps := pubsub.New()
	ps.Sub(func(s string) {
		fmt.Println(s)
	})

To publish:

	ps.Pub("hello world")

The message are allowed to pass any types, and passing to subscribers which
can accept the type for the argument of callback.
*/
package main

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"
)

type PubSubError struct {
	f interface{}
	e interface{}
}

func (pse *PubSubError) String() string {
	return fmt.Sprintf("%v: %v", pse.f, pse.e)
}

func (pse *PubSubError) Error() string {
	return fmt.Sprint(pse.e)
}

func (pse *PubSubError) Subscriber() interface{} {
	return pse.f
}

// PubSub contains channel and callbacks.
type PubSub struct {
	c chan interface{}
	f []interface{}
	m sync.Mutex
	e chan error
}

// New return new PubSub intreface.
func New() *PubSub {
	ps := new(PubSub)
	ps.c = make(chan interface{})
	ps.e = make(chan error)
	go func() {
		for v := range ps.c {
			rv := reflect.ValueOf(v)
			ps.m.Lock()
			for _, f := range ps.f {
				rf := reflect.ValueOf(f)
				if rv.Type() == reflect.ValueOf(f).Type().In(0) {
					go func(f interface{}, rf reflect.Value) {
						defer func() {
							if err := recover(); err != nil {
								ps.e <- &PubSubError{f, err}
							}
						}()
						rf.Call([]reflect.Value{rv})
					}(f, rf)
				}
			}
			ps.m.Unlock()
		}
	}()
	return ps
}

func (ps *PubSub) Error() chan error {
	return ps.e
}

// Sub subscribe to the PubSub.
func (ps *PubSub) Sub(f interface{}) error {
	rf := reflect.ValueOf(f)
	if rf.Kind() != reflect.Func {
		return errors.New("Not a function")
	}
	if rf.Type().NumIn() != 1 {
		return errors.New("Number of arguments should be 1")
	}
	ps.m.Lock()
	defer ps.m.Unlock()
	ps.f = append(ps.f, f)
	return nil
}

// Leave unsubscribe to the PubSub.
func (ps *PubSub) Leave(f interface{}) {
	var fp uintptr
	if f == nil {
		if pc, _, _, ok := runtime.Caller(1); ok {
			fp = runtime.FuncForPC(pc).Entry()
		}
	} else {
		fp = reflect.ValueOf(f).Pointer()
	}
	ps.m.Lock()
	defer ps.m.Unlock()
	result := make([]interface{}, 0, len(ps.f))
	last := 0
	for i, v := range ps.f {
		if reflect.ValueOf(v).Pointer() == fp {
			result = append(result, ps.f[last:i]...)
			last = i + 1
		}
	}
	ps.f = append(result, ps.f[last:]...)
}

// Pub publish to the PubSub.
func (ps *PubSub) Pub(v interface{}) {
	ps.c <- v
}

// Close closes PubSub. To inspect unbsubscribing for another subscruber, you must create message structure to notify them. After publish notifycations, Close should be called.
func (ps *PubSub) Close() {
	close(ps.c)
	ps.f = nil
}
