package controllers

import (
	"testing"
)

func Test_stateText(t *testing.T) {
	type args struct {
		status Status
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Should return Ready state test",
			args: args{status: Registered | Configured},
			want: "Ready",
		},
		{
			name: "Should return Processing state test",
			args: args{status: Processing},
			want: "Processing",
		},
		{
			name: "Should return Processing state test",
			args: args{status: Processing | Registered},
			want: "Processing",
		},
		{
			name: "Should return Processing state test",
			args: args{status: Processing | Configured},
			want: "Processing",
		},
		{
			name: "Should return Failed state test 1",
			args: args{status: Failed},
			want: "Failed",
		},
		{
			name: "Should return Failed state test 2",
			args: args{status: Processing | Failed},
			want: "Failed",
		},
		{
			name: "Should return Failed state test 3",
			args: args{status: Processing | Registered | Failed},
			want: "Failed",
		},
		{
			name: "Should return Failed state test 3",
			args: args{status: Processing | Configured | Failed},
			want: "Failed",
		},
		{
			name: "Should return Failed state test 3",
			args: args{status: Configured | Failed},
			want: "Failed",
		},
		{
			name: "Should return Failed state test 3",
			args: args{status: Registered | Failed},
			want: "Failed",
		},
		{
			name: "Should return Failed state test 4",
			args: args{status: Processing | Registered | Configured | Failed},
			want: "Failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stateText(tt.args.status); got != tt.want {
				t.Errorf("stateText() = %v, want %v", got, tt.want)
			}
		})
	}
}

/*func Test_statusNumber(t *testing.T) {
	type args struct {
		status v1beta1.CompassManagerMappingStatus
	}
	tests := []struct {
		name string
		args args
		want Status
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statusNumber(tt.args.status); got != tt.want {
				t.Errorf("statusNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}*/
