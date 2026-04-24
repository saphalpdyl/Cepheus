package processor_shared

import "context"

// interface shared by all processors
type Processor interface {
	Start(context.Context)
}
