//                           _       _
// __      _____  __ ___   ___  __ _| |_ ___
// \ \ /\ / / _ \/ _` \ \ / / |/ _` | __/ _ \
//  \ V  V /  __/ (_| |\ V /| | (_| | ||  __/
//   \_/\_/ \___|\__,_| \_/ |_|\__,_|\__\___|
//
//  Copyright © 2016 - 2024 Weaviate B.V. All rights reserved.
//
//  CONTACT: hello@weaviate.io
//

package config

import (
	"bytes"
	"io"
	"regexp"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4/json"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/weaviate/weaviate/usecases/config/runtime"
)

func TestParseRuntimeConfig(t *testing.T) {
	// parser should fail if any unknown fields exist in the file
	t.Run("parser should fail if any unknown fields exist in the file", func(t *testing.T) {
		// rationale: Catch and fail early if any typo on the config file.

		buf := []byte(`autoschema_enabled: true`)
		cfg, err := ParseRuntimeConfig(buf)
		require.NoError(t, err)
		assert.Equal(t, true, cfg.AutoschemaEnabled.Get())

		buf = []byte(`autoschema_enbaled: false`) // note: typo.
		cfg, err = ParseRuntimeConfig(buf)
		require.ErrorContains(t, err, "autoschema_enbaled") // should contain misspelled field
		assert.Nil(t, cfg)
	})

	t.Run("YAML tag should be lower_snake_case", func(t *testing.T) {
		var r WeaviateRuntimeConfig

		jd, err := json.Marshal(r)
		require.NoError(t, err)

		var vv map[string]any
		require.NoError(t, json.Unmarshal(jd, &vv))

		for k := range vv {
			// check if all the keys lower_snake_case.
			assertConfigKey(t, k)
		}
	})

	t.Run("JSON tag should be lower_snake_case in the runtime config", func(t *testing.T) {
		var r WeaviateRuntimeConfig

		yd, err := yaml.Marshal(r)
		require.NoError(t, err)

		var vv map[string]any
		require.NoError(t, yaml.Unmarshal(yd, &vv))

		for k := range vv {
			// check if all the keys lower_snake_case.
			assertConfigKey(t, k)
		}
	})
}

// assertConfigKey asserts if the `yaml` key is standard `lower_snake_case` (e.g: not `UPPER_CASE`)
func assertConfigKey(t *testing.T, key string) {
	t.Helper()

	re := regexp.MustCompile(`^[a-z]+(_[a-z]+)*$`)
	if !re.MatchString(key) {
		t.Fatalf("given key %v is not lower snake case. The json/yaml tag for runtime config should be all lower snake case (e.g my_key, not MY_KEY)", key)
	}
}

func TestUpdateRuntimeConfig(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	t.Run("updating should reflect changes in registered configs", func(t *testing.T) {
		var (
			colCount                 runtime.DynamicValue[int]
			autoSchema               runtime.DynamicValue[bool]
			asyncRep                 runtime.DynamicValue[bool]
			readLogLevel             runtime.DynamicValue[string]
			writeLogLevel            runtime.DynamicValue[string]
			revectorizeCheckDisabled runtime.DynamicValue[bool]
			minFinWait               runtime.DynamicValue[time.Duration]
		)

		reg := &WeaviateRuntimeConfig{
			MaximumAllowedCollectionsCount:  &colCount,
			AutoschemaEnabled:               &autoSchema,
			AsyncReplicationDisabled:        &asyncRep,
			TenantActivityReadLogLevel:      &readLogLevel,
			TenantActivityWriteLogLevel:     &writeLogLevel,
			RevectorizeCheckDisabled:        &revectorizeCheckDisabled,
			ReplicaMovementMinimumAsyncWait: &minFinWait,
		}

		// parsed from yaml configs for example
		buf := []byte(`autoschema_enabled: true
maximum_allowed_collections_count: 13
replica_movement_minimum_async_wait: 10s`)
		parsed, err := ParseRuntimeConfig(buf)
		require.NoError(t, err)

		// before update (zero values)
		assert.Equal(t, false, autoSchema.Get())
		assert.Equal(t, 0, colCount.Get())
		assert.Equal(t, 0*time.Second, minFinWait.Get())

		require.NoError(t, UpdateRuntimeConfig(log, reg, parsed))

		// after update (reflect from parsed values)
		assert.Equal(t, true, autoSchema.Get())
		assert.Equal(t, 13, colCount.Get())
		assert.Equal(t, 10*time.Second, minFinWait.Get())
	})

	t.Run("Reset() of non-exist config values in parsed yaml shouldn't panic", func(t *testing.T) {
		var (
			colCount   runtime.DynamicValue[int]
			autoSchema runtime.DynamicValue[bool]
			// leaving out `asyncRep` config
		)

		reg := &WeaviateRuntimeConfig{
			MaximumAllowedCollectionsCount: &colCount,
			AutoschemaEnabled:              &autoSchema,
			// leaving out `asyncRep` config
		}

		// parsed from yaml configs for example
		buf := []byte(`autoschema_enabled: true
maximum_allowed_collections_count: 13`) // leaving out `asyncRep` config
		parsed, err := ParseRuntimeConfig(buf)
		require.NoError(t, err)

		// before update (zero values)
		assert.Equal(t, false, autoSchema.Get())
		assert.Equal(t, 0, colCount.Get())

		require.NotPanics(t, func() { UpdateRuntimeConfig(log, reg, parsed) })

		// after update (reflect from parsed values)
		assert.Equal(t, true, autoSchema.Get())
		assert.Equal(t, 13, colCount.Get())
	})

	t.Run("SetValue on nil receiver should not panic", func(t *testing.T) {
		var (
			nilInt    *runtime.DynamicValue[int]
			nilFloat  *runtime.DynamicValue[float64]
			nilBool   *runtime.DynamicValue[bool]
			nilDur    *runtime.DynamicValue[time.Duration]
			nilString *runtime.DynamicValue[string]
		)

		// These should not panic
		require.NotPanics(t, func() {
			nilInt.SetValue(42)
		})
		require.NotPanics(t, func() {
			nilFloat.SetValue(3.14)
		})
		require.NotPanics(t, func() {
			nilBool.SetValue(true)
		})
		require.NotPanics(t, func() {
			nilDur.SetValue(5 * time.Second)
		})
		require.NotPanics(t, func() {
			nilString.SetValue("test")
		})

		// Values should remain unchanged (still return zero values)
		assert.Equal(t, 0, nilInt.Get())
		assert.Equal(t, float64(0), nilFloat.Get())
		assert.Equal(t, false, nilBool.Get())
		assert.Equal(t, time.Duration(0), nilDur.Get())
		assert.Equal(t, "", nilString.Get())
	})

	t.Run("SetValue on initialized receiver should work normally", func(t *testing.T) {
		dInt := runtime.NewDynamicValue(10)
		dFloat := runtime.NewDynamicValue(2.5)
		dBool := runtime.NewDynamicValue(false)
		dDur := runtime.NewDynamicValue(1 * time.Second)
		dString := runtime.NewDynamicValue("initial")

		// Set new values
		dInt.SetValue(20)
		dFloat.SetValue(3.14)
		dBool.SetValue(true)
		dDur.SetValue(2 * time.Second)
		dString.SetValue("updated")

		// Values should be updated
		assert.Equal(t, 20, dInt.Get())
		assert.Equal(t, 3.14, dFloat.Get())
		assert.Equal(t, true, dBool.Get())
		assert.Equal(t, 2*time.Second, dDur.Get())
		assert.Equal(t, "updated", dString.Get())
	})
	t.Run("updating config should split out corresponding log lines", func(t *testing.T) {
		log := logrus.New()
		logs := bytes.Buffer{}
		log.SetOutput(&logs)

		var (
			colCount   = runtime.NewDynamicValue(7)
			autoSchema runtime.DynamicValue[bool]
		)

		reg := &WeaviateRuntimeConfig{
			MaximumAllowedCollectionsCount: colCount,
			AutoschemaEnabled:              &autoSchema,
		}

		// parsed from yaml configs for example
		buf := []byte(`autoschema_enabled: true
maximum_allowed_collections_count: 13`) // leaving out `asyncRep` config
		parsed, err := ParseRuntimeConfig(buf)
		require.NoError(t, err)

		// before update (zero values)
		assert.Equal(t, false, autoSchema.Get())
		assert.Equal(t, 7, colCount.Get())

		require.NoError(t, UpdateRuntimeConfig(log, reg, parsed))
		assert.Contains(t, logs.String(), `level=info msg="runtime overrides: config 'MaximumAllowedCollectionsCount' changed from '7' to '13'" action=runtime_overrides_changed field=MaximumAllowedCollectionsCount new_value=13 old_value=7`)
		assert.Contains(t, logs.String(), `level=info msg="runtime overrides: config 'AutoschemaEnabled' changed from 'false' to 'true'" action=runtime_overrides_changed field=AutoschemaEnabled new_value=true old_value=false`)
		logs.Reset()

		// change configs
		buf = []byte(`autoschema_enabled: false
maximum_allowed_collections_count: 10`)
		parsed, err = ParseRuntimeConfig(buf)
		require.NoError(t, err)

		require.NoError(t, UpdateRuntimeConfig(log, reg, parsed))
		assert.Contains(t, logs.String(), `level=info msg="runtime overrides: config 'MaximumAllowedCollectionsCount' changed from '13' to '10'" action=runtime_overrides_changed field=MaximumAllowedCollectionsCount new_value=10 old_value=13`)
		assert.Contains(t, logs.String(), `level=info msg="runtime overrides: config 'AutoschemaEnabled' changed from 'true' to 'false'" action=runtime_overrides_changed field=AutoschemaEnabled new_value=false old_value=true`)
		logs.Reset()

		// remove configs (`maximum_allowed_collections_count`)
		buf = []byte(`autoschema_enabled: false`)
		parsed, err = ParseRuntimeConfig(buf)
		require.NoError(t, err)

		require.NoError(t, UpdateRuntimeConfig(log, reg, parsed))
		assert.Contains(t, logs.String(), `level=info msg="runtime overrides: config 'MaximumAllowedCollectionsCount' changed from '10' to '7'" action=runtime_overrides_changed field=MaximumAllowedCollectionsCount new_value=7 old_value=10`)
	})

	t.Run("updating priorities", func(t *testing.T) {
		// invariants:
		// 1. If field doesn't exist, should return default value
		// 2. If field exist, but removed next time, should return default value not the old value.

		var (
			colCount                 runtime.DynamicValue[int]
			autoSchema               runtime.DynamicValue[bool]
			asyncRep                 runtime.DynamicValue[bool]
			readLogLevel             runtime.DynamicValue[string]
			writeLogLevel            runtime.DynamicValue[string]
			revectorizeCheckDisabled runtime.DynamicValue[bool]
			minFinWait               runtime.DynamicValue[time.Duration]
		)

		reg := &WeaviateRuntimeConfig{
			MaximumAllowedCollectionsCount:  &colCount,
			AutoschemaEnabled:               &autoSchema,
			AsyncReplicationDisabled:        &asyncRep,
			TenantActivityReadLogLevel:      &readLogLevel,
			TenantActivityWriteLogLevel:     &writeLogLevel,
			RevectorizeCheckDisabled:        &revectorizeCheckDisabled,
			ReplicaMovementMinimumAsyncWait: &minFinWait,
		}

		// parsed from yaml configs for example
		buf := []byte(`autoschema_enabled: true
maximum_allowed_collections_count: 13
replica_movement_minimum_async_wait: 10s`)
		parsed, err := ParseRuntimeConfig(buf)
		require.NoError(t, err)

		// before update (zero values)
		assert.Equal(t, false, autoSchema.Get())
		assert.Equal(t, 0, colCount.Get())
		assert.Equal(t, false, asyncRep.Get()) // this field doesn't exist in original config file.
		assert.Equal(t, 0*time.Second, minFinWait.Get())

		require.NoError(t, UpdateRuntimeConfig(log, reg, parsed))

		// after update (reflect from parsed values)
		assert.Equal(t, true, autoSchema.Get())
		assert.Equal(t, 13, colCount.Get())
		assert.Equal(t, false, asyncRep.Get()) // this field doesn't exist in original config file, should return default value.
		assert.Equal(t, 10*time.Second, minFinWait.Get())

		// removing `maximum_allowed_collection_count` from config
		buf = []byte(`autoschema_enabled: false`)
		parsed, err = ParseRuntimeConfig(buf)
		require.NoError(t, err)

		// before update. Should have old values
		assert.Equal(t, true, autoSchema.Get())
		assert.Equal(t, 13, colCount.Get())
		assert.Equal(t, false, asyncRep.Get()) // this field doesn't exist in original config file, should return default value.

		require.NoError(t, UpdateRuntimeConfig(log, reg, parsed))

		// after update.
		assert.Equal(t, false, autoSchema.Get())
		assert.Equal(t, 0, colCount.Get())     // this should still return `default` value. not old value
		assert.Equal(t, false, asyncRep.Get()) // this field doesn't exist in original config file, should return default value.
	})
}
