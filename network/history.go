// Package network history.go provides ring buffer for storing historical data
// like network throughput samples for sparkline graphs.
package network

// RingBuffer is a fixed-size circular buffer for storing float64 values.
// When full, new values overwrite the oldest ones.
// Thread-safe for single-writer, single-reader scenarios.
type RingBuffer struct {
	data  []float64
	size  int
	head  int // Next write position
	count int // Number of valid elements
}

// NewRingBuffer creates a new ring buffer with the specified capacity.
// Capacity should match how many historical samples you want to keep.
// For 20 seconds of history at 1s intervals, use capacity=20.
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		data: make([]float64, capacity),
		size: capacity,
	}
}

// Add appends a value to the buffer, overwriting oldest if full.
func (r *RingBuffer) Add(value float64) {
	r.data[r.head] = value
	r.head = (r.head + 1) % r.size
	if r.count < r.size {
		r.count++
	}
}

// GetAll returns all values in order from oldest to newest.
// Returns a slice of length count (may be less than capacity if not full).
func (r *RingBuffer) GetAll() []float64 {
	if r.count == 0 {
		return nil
	}

	result := make([]float64, r.count)

	if r.count < r.size {
		// Buffer not yet full, data starts at index 0
		copy(result, r.data[:r.count])
	} else {
		// Buffer is full, oldest is at head position
		// Copy from head to end, then from start to head
		firstPart := r.size - r.head
		copy(result[:firstPart], r.data[r.head:])
		copy(result[firstPart:], r.data[:r.head])
	}

	return result
}

// Len returns the number of elements currently in the buffer.
func (r *RingBuffer) Len() int {
	return r.count
}

// Clear resets the buffer to empty state.
func (r *RingBuffer) Clear() {
	r.head = 0
	r.count = 0
}

// Last returns the most recently added value, or 0 if empty.
func (r *RingBuffer) Last() float64 {
	if r.count == 0 {
		return 0
	}
	lastIdx := (r.head - 1 + r.size) % r.size
	return r.data[lastIdx]
}

// Max returns the maximum value in the buffer, or 0 if empty.
func (r *RingBuffer) Max() float64 {
	if r.count == 0 {
		return 0
	}

	maxVal := r.data[0]
	for i := 1; i < r.count; i++ {
		idx := i
		if r.count == r.size {
			idx = (r.head + i) % r.size
		}
		if r.data[idx] > maxVal {
			maxVal = r.data[idx]
		}
	}
	return maxVal
}

// Average returns the average of all values in the buffer, or 0 if empty.
func (r *RingBuffer) Average() float64 {
	if r.count == 0 {
		return 0
	}

	var sum float64
	for _, v := range r.GetAll() {
		sum += v
	}
	return sum / float64(r.count)
}

