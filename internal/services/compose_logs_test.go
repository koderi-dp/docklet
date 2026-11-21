package services

import "testing"

func TestPerContainerTail(t *testing.T) {
	cases := []struct {
		name             string
		tail, containers int
		want             int
	}{
		{"one container gets the full tail", 1000, 1, 1000},
		{"two containers get the whole tail", 1000, 2, 1000},
		{"four containers", 1000, 4, 500},
		{"ten containers", 5000, 10, 1000},
		{"ceil", 1000, 3, 667},
		{"floor", 1000, 20, 200},
		{"floor with many containers", 5000, 100, 200},
		{"capped at tail under the floor", 100, 5, 100},
		{"zero containers", 1000, 0, 1000},
		{"negative containers", 1000, -3, 1000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := perContainerTail(c.tail, c.containers); got != c.want {
				t.Fatalf("perContainerTail(%d, %d) = %d, want %d", c.tail, c.containers, got, c.want)
			}
		})
	}
}
