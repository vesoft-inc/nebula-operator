package hash

import "testing"

func TestHash(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "text is empty",
			want: "cf83e1357eefb8bd",
		},
		{
			name: "text not empty",
			text: "Hello world",
			want: "b7f783baed8297f0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Hash(tt.text); got != tt.want {
				t.Errorf("Hash() = %v, want %v", got, tt.want)
			}
		})
	}
}
