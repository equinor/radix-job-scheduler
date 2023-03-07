package radix

import (
	"testing"

	radixclientfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
)

func Test_RadixBatchWatcher(t *testing.T) {
	type args struct {
		namespace string
		notifier  Notifier
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
			radixClient := radixclientfake.NewSimpleClientset()

			NewRadixBatchWatcher(radixClient, tt.args.namespace, tt.args.notifier)
		})
	}
}
