// The `basetextoutputsegment` serves as a basis for segments implementing the TextOutputSegment-interface.
// It extends the BaseSegment by adding a file parameter that is used for outputing data in segment inheriting this base segment
// The file name for this output file should be defined via the parameter `filename` otherise StdOut will be used
package basetextoutputsegment

import (
	"os"

	"codeberg.org/BelWue/flowpipeline/segments/base/basesegment"
)

// An extended basis for Segment implementations in the filter group. It
// contains the necessities to process filtered (dropped) flows.
type BaseTextOutputSegment struct {
	basesegment.BaseSegment
	File *os.File // optional, default is empty which means stdout
}

func (s *BaseTextOutputSegment) GetOutput(config map[string]string) (*os.File, error) {
	var err error
	if config["filename"] != "" {
		s.File, err = os.Create(config["filename"])
		if err != nil {
			return nil, err
		}
	} else {
		s.File = os.Stdout
	}
	return s.File, nil
}
