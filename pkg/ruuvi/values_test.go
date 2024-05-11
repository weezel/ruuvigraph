package ruuvi

import (
	"testing"
)

func TestReadConfig(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		want    map[string]string
		args    args
		wantErr bool
	}{
		{
			name: "",
			args: args{
				filename: "example_devices.conf",
			},
			want: map[string]string{
				"d8:82:aa:bb:cc:dd": "Kitchen",
				"fc:8a:aa:bb:cc:dd": "Balcony",
				"cb:15:aa:bb:cc:dd": "Bedroom",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadAliases(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for key, val := range tt.want {
				foundVal, ok := got[key]
				if !ok {
					t.Errorf("ReadConfig() missing key from the result set: %s", key)
				}
				if foundVal != val {
					t.Errorf("ReadConfig() values differ: got=%s, want=%s", foundVal, val)
				}
			}
		})
	}
}
