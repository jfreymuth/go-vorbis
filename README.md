# go-vorbis
a native go ogg vorbis decoder

## Usage

	v, err := vorbis.Open(reader)
	// handle error

	for {
		out, err := v.DecodePacket()
		if err == io.EOF {
			break
		} else if err != nil {
			// handle error
		}
		// do something with out
	}