package radix

import (
	"fmt"
	"github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_RadixBatchWatcher(t *testing.T) {
	type args struct {
		radixClient versioned.Interface
		namespace   string
		notifier    Notifier
	}
	tests := []struct {
		name    string
		args    args
		want    *Watcher
		wantErr assert.ErrorAssertionFunc
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewRadixBatchWatcher(tt.args.radixClient, tt.args.namespace, tt.args.notifier)
			if !tt.wantErr(t, err, fmt.Sprintf("NewRadixBatchWatcher(%v, %v, %v)", tt.args.radixClient, tt.args.namespace, tt.args.notifier)) {
				return
			}
			assert.Equalf(t, tt.want, got, "NewRadixBatchWatcher(%v, %v, %v)", tt.args.radixClient, tt.args.namespace, tt.args.notifier)
		})
	}
}
