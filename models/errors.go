package models

import (
	"fmt"

)

type StatusFractal int

const (
	StopProcessByUser            StatusFractal = 11
	StopProcessByCommandUser     StatusFractal = 12
	StopProcessByTimeoutWorkflow StatusFractal = 13
	FailedProcessWithError       StatusFractal = 14
	NoAnyRespond                 StatusFractal = 15
	InternalError                StatusFractal = 16
	InflowStopSignal             StatusFractal = 17
	FailedInitInternalError      StatusFractal = 18
	InfraNoRespond               StatusFractal = 19
	InfraBadRequest              StatusFractal = 20
	ProcessBadRequest            StatusFractal = 21
	ProcessLimit                 StatusFractal = 22
	ProcessStopped               StatusFractal = 23
	ProcessNodeDataBadRequest    StatusFractal = 24
)

type FractalError struct {
	Code    StatusFractal
	Message string
}

func (e FractalError) Error() string {
	return fmt.Sprintf("code: %d, message: %s", int(e.Code), e.Message)
}

func NewFractalError(code StatusFractal, message string) FractalError {
	return FractalError{
		Code:    code,
		Message: message,
	}
}

func NewInternalError(message string) error {

	return NewFractalError(InternalError, message)
}

func NewInfraError(message string) error {

	return NewFractalError(InfraNoRespond, message)
}
func NewInfraBadRequestError(message string) error {

	return NewFractalError(InfraBadRequest, message)
}


