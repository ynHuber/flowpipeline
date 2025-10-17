// The DeprecationWrapper can be used to create a clone of a segment that logs warnings when the segment is initialized or run.
// This enables registering a copy of a segment for deprecated segment names, allowing a none breaking deprecation of old segment names.
package segments

import (
	"sync"

	"github.com/rs/zerolog/log"
)

type segmentDeprecationWrapper struct {
	Segment

	oldName string
	newName string
}

func CreateSegmentDeprecationWrapper(segment Segment, oldName string, newName string) Segment {
	return &segmentDeprecationWrapper{
		Segment: segment,
		oldName: oldName,
		newName: newName,
	}
}

func (segment segmentDeprecationWrapper) New(config map[string]string) Segment {
	log.Warn().Msg("Using deprected segment name '" + segment.oldName + "'. Please use '" + segment.newName + "' instead")
	return segment.Segment.New(config)
}

func (segment *segmentDeprecationWrapper) Run(wg *sync.WaitGroup) {
	log.Warn().Msg("Using deprected segment name '" + segment.oldName + "'. Please use '" + segment.newName + "' instead")
	segment.Segment.Run(wg)
}
