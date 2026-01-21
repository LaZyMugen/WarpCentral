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