package status

import (
	"testing"

	"github.com/kyma-project/compass-manager/api/v1beta1"
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
			name: "Should return Ready state test from Registered and Configured status",
			args: args{status: Registered | Configured},
			want: "Ready",
		},
		{
			name: "Should return Processing state test from Processing status",
			args: args{status: Processing},
			want: "Processing",
		},
		{
			name: "Should return Processing state test from Processing and Registered status",
			args: args{status: Processing | Registered},
			want: "Processing",
		},
		{
			name: "Should return Processing state test from Processing and Configured status",
			args: args{status: Processing | Configured},
			want: "Processing",
		},
		{
			name: "Should return Failed state from Failed status",
			args: args{status: Failed},
			want: "Failed",
		},
		{
			name: "Should return Failed state from Processing and Failed status",
			args: args{status: Processing | Failed},
			want: "Failed",
		},
		{
			name: "Should return Failed state from Processing Registered and Failed status",
			args: args{status: Processing | Registered | Failed},
			want: "Failed",
		},
		{
			name: "Should return Failed state from Processing Configured and Failed status",
			args: args{status: Processing | Configured | Failed},
			want: "Failed",
		},
		{
			name: "Should return Failed state from Configured and Failed status",
			args: args{status: Configured | Failed},
			want: "Failed",
		},
		{
			name: "Should return Failed state from Registered and Failed status",
			args: args{status: Registered | Failed},
			want: "Failed",
		},
		{
			name: "Should return Failed state from Processing Registered Configured and Failed status",
			args: args{status: Processing | Registered | Configured | Failed},
			want: "Failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StateText(tt.args.status); got != tt.want {
				t.Errorf("stateText() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_statusNumber(t *testing.T) {
	type args struct {
		status v1beta1.CompassManagerMappingStatus
	}
	tests := []struct {
		name string
		args args
		want Status
	}{
		{
			name: "Should return Empty status number from initially created status",
			args: args{status: v1beta1.CompassManagerMappingStatus{}},
			want: Empty,
		},
		{
			name: "Should return Processing status number from Processing status",
			args: args{status: v1beta1.CompassManagerMappingStatus{State: "Processing"}},
			want: Processing,
		},
		{
			name: "Should return Failed status number from Failed status",
			args: args{status: v1beta1.CompassManagerMappingStatus{State: "Failed"}},
			want: Failed,
		},
		{
			name: "Should return Registered status number from Registered status",
			args: args{status: v1beta1.CompassManagerMappingStatus{Registered: true}},
			want: Registered,
		},
		{
			name: "Should return Configured status number from Configured status",
			args: args{status: v1beta1.CompassManagerMappingStatus{Configured: true}},
			want: Configured,
		},
		{
			name: "Should return  Configured | Registered status number from Configured and Registered status",
			args: args{status: v1beta1.CompassManagerMappingStatus{Configured: true, Registered: true}},
			want: Configured | Registered,
		},
		{
			name: "Should return Registered | Failed status number from Registered and Failed status",
			args: args{status: v1beta1.CompassManagerMappingStatus{State: "Failed", Registered: true}},
			want: Registered | Failed,
		},
		{
			name: "Should return Registered | Processing status number from Registered and Processing status",
			args: args{status: v1beta1.CompassManagerMappingStatus{State: "Processing", Registered: true}},
			want: Registered | Processing,
		},
		{
			name: "Should return Configured | Failed status number from Registered and Failed status",
			args: args{status: v1beta1.CompassManagerMappingStatus{State: "Failed", Configured: true}},
			want: Configured | Failed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Number(tt.args.status); got != tt.want {
				t.Errorf("statusNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}
