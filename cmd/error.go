package cmd

import "errors"

var (
	ErrEmptyAwsTfstateBucket        = errors.New("empty aws tfstate bucket")
	ErrInvalidTfstateSourceSelector = errors.New("invalid tfstate source selector")
)
