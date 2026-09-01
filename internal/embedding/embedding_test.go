package embedding

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestProfileIsStableAndConfigurationSpecific(t *testing.T) {
	first := Profile("model", "v1", 3)
	if first != Profile("model", "v1", 3) {
		t.Fatal("profile is not deterministic")
	}
	if first == Profile("model", "v2", 3) ||
		first == Profile("model", "v1", 4) {
		t.Fatal("profile did not change with embedding configuration")
	}
}

func TestWindowsUsesDeterministicOverlap(t *testing.T) {
	got := Windows("one two three four five six", 14, 4)
	want := []string{"one two three", "three four", "four five six"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("windows = %#v, want %#v", got, want)
	}
}

func TestAverageNormalizesVectors(t *testing.T) {
	vector, err := Average([][]float32{
		{1, 0},
		{0, 1},
	}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(float64(vector[0]-0.70710677)) > 0.0001 ||
		math.Abs(float64(vector[1]-0.70710677)) > 0.0001 {
		t.Fatalf("unexpected average vector: %#v", vector)
	}
}

func TestAverageRejectsInvalidVectors(t *testing.T) {
	if _, err := Average([][]float32{{1}}, 2); !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("error = %v", err)
	}
	if _, err := Average([][]float32{{0, 0}}, 2); !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("error = %v", err)
	}
}
