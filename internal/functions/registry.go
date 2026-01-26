package functions

import (
	"fmt"
	"reflect"
)

type FunctionType func(args ...interface{}) (string, error)

var FunctionRegistry = map[string]FunctionType{}

func RegisterFunction(name string, fn FunctionType) {
	FunctionRegistry[name] = fn
}

func Get(name string) FunctionType {
	return FunctionRegistry[name]
}

func RegisterGoFunction(name string, fn interface{}) error {
	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		return fmt.Errorf("function '%s' is not a function", name)
	}

	wrapper := func(args ...interface{}) (string, error) {
		numIn := fnType.NumIn()
		
		if numIn == 0 {
			result := fnValue.Call(nil)
			if len(result) == 1 {
				return result[0].String(), nil
			} else if len(result) == 2 {
				if !result[1].IsNil() {
					return "", result[1].Interface().(error)
				}
				return result[0].String(), nil
			}
			return "", fmt.Errorf("function must return string or (string, error)")
		}

		callArgs := make([]reflect.Value, numIn)
		for i := 0; i < numIn; i++ {
			paramType := fnType.In(i)
			
			if i < len(args) {
				argValue := reflect.ValueOf(args[i])
				if argValue.Type().AssignableTo(paramType) {
					callArgs[i] = argValue
				} else if argValue.Type().ConvertibleTo(paramType) {
					callArgs[i] = argValue.Convert(paramType)
				} else {
					return "", fmt.Errorf("cannot convert argument %d (type %s) to type %s", i, argValue.Type(), paramType)
				}
			} else {
				callArgs[i] = reflect.Zero(paramType)
			}
		}

		result := fnValue.Call(callArgs)

		if len(result) == 1 {
			return result[0].String(), nil
		} else if len(result) == 2 {
			if !result[1].IsNil() {
				return "", result[1].Interface().(error)
			}
			return result[0].String(), nil
		}

		return "", fmt.Errorf("function must return string or (string, error)")
	}

	FunctionRegistry[name] = wrapper
	return nil
}
