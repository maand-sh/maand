package kv

import "fmt"

type NotFoundError struct {
	Namespace string
	Key       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("key %s not found in namespace %s", e.Key, e.Namespace)
}

func NewNotFoundError(namespace, key string) *NotFoundError {
	return &NotFoundError{
		Namespace: namespace,
		Key:       key,
	}
}
