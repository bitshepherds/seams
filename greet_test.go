package sheepdogseams

import "testing"

func TestGreetSaysHelloToTheNameItIsGiven(t *testing.T) {
	if got := Greet("Ada"); got != "Hello, Ada!" {
		t.Fatalf("Greet(\"Ada\") = %q", got)
	}
}
