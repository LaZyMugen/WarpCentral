package chunker



type Chunk struct{
	ID int;
	Start int64;
	End int64
			
}
	
  func Split(totalSize int64, parts int) []Chunk {
	if parts <= 0 {
		parts = 1
	}
	if totalSize <= 0 {
		return nil
  }

  chunks := make([]Chunk, 0, parts)
	partSize := totalSize / int64(parts)
	rem := totalSize % int64(parts)

	var start int64 = 0
	for i := 0; i < parts; i++ {
		size := partSize
		if int64(i) < rem {
			size++
		}
		end := start + size - 1
		chunks = append(chunks, Chunk{
			ID:    i,
			Start: start,
			End:   end,
		})
		start = end + 1
	}

	return chunks
}