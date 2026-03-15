package vectorindex

// Embedder produces dense vector representations of text.
type Embedder interface {
	// Embed returns a dense vector representation of the text.
	Embed(text string) ([]float32, error)

	// Dimension returns the output vector dimensionality.
	Dimension() int

	// Close releases any resources held by the embedder.
	Close() error
}
