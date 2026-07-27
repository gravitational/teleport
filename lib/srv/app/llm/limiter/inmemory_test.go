// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package limiter

import (
	"fmt"
	"math"
	"testing"
	"unicode"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/gravitational/teleport/api/types"
)

// TestProperty_OnlyOneUsageRequestType given a reserve request, ensures that
// only one usage type can be used.
func TestInMemoryOnlyOneUsageRequestType(t *testing.T) {
	l := NewInMemory(0, 0)
	_, _, err := l.Reserve(t.Context(), ReserveRequest{
		Usage: &Usage{
			InputTokens:  32,
			OutputTokens: 32,
		},
		MaxUsage: &Usage{
			InputTokens:  32,
			OutputTokens: 32,
		},
	})
	require.Error(t, err)
}

// TestProperty_InputOutputIsolated given a limiter, input and output limits
// are isolated and do not interfere on each other.
func TestProperty_InputOutputIsolated(t *testing.T) {
	t.Run("input", rapid.MakeCheck(func(t *rapid.T) {
		app := appGenerator().Draw(t, "app")
		baseLimits := rapid.UintRange(32, 128).Draw(t, "base_limits")
		// Make the output limits larger so requests will exceed input limits
		// first.
		l := NewInMemory(baseLimits, baseLimits*2)

		nReservations := rapid.IntRange(1, int(baseLimits)).Draw(t, "n_reservations")
		for _, usage := range usageGenerator(nReservations, baseLimits, baseLimits).Draw(t, "reservations") {
			_ = reserveAndSettle(t, l, app, usage)
		}
		_, settleFunc, err := l.Reserve(t.Context(), ReserveRequest{App: app, Usage: &Usage{
			InputTokens:  rapid.Just(uint(1)).Draw(t, "input_usage"),
			OutputTokens: rapid.UintRange(1, baseLimits-1).Draw(t, "output_usage"),
		}})
		require.Error(t, err)
		require.NotNil(t, settleFunc)
	}))

	t.Run("output", rapid.MakeCheck(func(t *rapid.T) {
		app := appGenerator().Draw(t, "app")
		baseLimits := rapid.UintRange(32, 128).Draw(t, "base_limits")
		// Make the input limits larger so requests will exceed input limits
		// first.
		l := NewInMemory(baseLimits*2, baseLimits)

		nReservations := rapid.IntRange(1, int(baseLimits)).Draw(t, "n_reservations")
		for _, usage := range usageGenerator(nReservations, baseLimits, baseLimits).Draw(t, "reservations") {
			_ = reserveAndSettle(t, l, app, usage)
		}
		_, settleFunc, err := l.Reserve(t.Context(), ReserveRequest{App: app, Usage: &Usage{
			InputTokens:  rapid.UintRange(1, baseLimits-1).Draw(t, "input_usage"),
			OutputTokens: rapid.Just(uint(1)).Draw(t, "output_usage"),
		}})
		require.Error(t, err)
		require.NotNil(t, settleFunc)
	}))
}

// TestProperty_AppsIsolated given a list of apps being served, their limits are
// isolated and do not affect each other.
func TestProperty_AppsIsolated(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		maxInput, maxOutput := genLimits.Draw(t, "max_input"), genLimits.Draw(t, "max_output")
		l := NewInMemory(maxInput, maxOutput)
		apps := rapid.SliceOfDistinct(appGenerator(), func(app types.Application) string { return app.GetName() }).Draw(t, "apps")
		randomApps := rapid.Permutation(apps)

		// For each app we'll make requests to consume the limit, leaving no
		// room.
		for _, app := range randomApps.Draw(t, "apps_reservations_permutation") {
			// Draw usage but not enough to exceed the app limit.
			var (
				totalInput  uint
				totalOutput uint
			)
			for _, usage := range usageGenerator(rapid.IntRange(1, 10).Draw(t, "n_app_reservations"), maxInput, maxOutput).Draw(t, "app_usage") {
				_, settleFunc, err := l.Reserve(t.Context(), ReserveRequest{App: app, Usage: &usage})
				require.NoError(t, err)
				require.NotNil(t, settleFunc)
				settleFunc(t.Context(), usage)
				totalInput += usage.InputTokens
				totalOutput += usage.OutputTokens
			}

			require.Equal(t, totalInput, l.Consumption(app).InputTokens)
			require.Equal(t, totalOutput, l.Consumption(app).OutputTokens)
		}

		// Now we'll do the last reservation that will return limit exceeded
		// for all apps.
		for _, app := range randomApps.Draw(t, "apps_reservations_permutation_failure") {
			_, settleFunc, err := l.Reserve(t.Context(), ReserveRequest{App: app, Usage: &Usage{InputTokens: 1, OutputTokens: 1}})
			require.Error(t, err)
			require.NotNil(t, settleFunc)
		}

	})
}

// TestProperty_ReserveInfo given a reservation with max usage, it returns the
// reserved output tokens.
func TestProperty_MaxUsageReportsReserveInfo(t *testing.T) {
	for name, tc := range map[string]struct {
		maxOutput  *rapid.Generator[uint]
		reserveReq func(maxOutput uint) *rapid.Generator[ReserveRequest]
		expectErr  require.ErrorAssertionFunc
		expectInfo func(tt require.TestingT, maxOutput uint, req ReserveRequest, info ReserveInfo)
	}{
		"requested value with limits": {
			maxOutput: genLimits,
			reserveReq: func(maxOutput uint) *rapid.Generator[ReserveRequest] {
				return rapid.Custom(func(t *rapid.T) ReserveRequest {
					return ReserveRequest{
						App: appGenerator().Draw(t, "app"),
						MaxUsage: &Usage{
							OutputTokens: rapid.UintMin(maxOutput).Draw(t, "max_provider_output"),
						},
					}
				})
			},
			expectErr: require.NoError,
			expectInfo: func(tt require.TestingT, maxOutput uint, req ReserveRequest, info ReserveInfo) {
				require.Equal(tt, maxOutput, info.OutputTokens)
			},
		},
		"provider defaults": {
			maxOutput: genLimits,
			reserveReq: func(maxOutput uint) *rapid.Generator[ReserveRequest] {
				return rapid.Custom(func(t *rapid.T) ReserveRequest {
					return ReserveRequest{
						App: appGenerator().Draw(t, "app"),
						MaxUsage: &Usage{
							OutputTokens: rapid.UintRange(1, maxOutput).Draw(t, "max_provider_output"),
						},
					}
				})
			},
			expectErr: require.NoError,
			expectInfo: func(tt require.TestingT, _ uint, req ReserveRequest, info ReserveInfo) {
				require.Equal(tt, req.MaxUsage.OutputTokens, info.OutputTokens)
			},
		},
		"not enough limit available": {
			maxOutput: rapid.Just(uint(0)),
			reserveReq: func(maxOutput uint) *rapid.Generator[ReserveRequest] {
				return rapid.Custom(func(t *rapid.T) ReserveRequest {
					return ReserveRequest{
						App: appGenerator().Draw(t, "app"),
						MaxUsage: &Usage{
							OutputTokens: rapid.Uint().Draw(t, "max_provider_output"),
						},
					}
				})
			},
			expectErr: require.Error,
		},
	} {
		t.Run(name, rapid.MakeCheck(func(t *rapid.T) {
			maxOutput := tc.maxOutput.Draw(t, "max_output")
			l := NewInMemory(genLimits.Draw(t, "max_input"), maxOutput)

			req := tc.reserveReq(maxOutput).Draw(t, "reserve_req")
			info, _, err := l.Reserve(t.Context(), req)
			tc.expectErr(t, err)
			if tc.expectInfo != nil {
				tc.expectInfo(t, maxOutput, req, info)
			}
		}))
	}
}

// TestProperty_EmptyUsageSettlementDoesNothing given a reservation that is
// settled with empty usage, it should not change the consumed values.
func TestProperty_EmptyUsageSettlementReturnToOriginalState(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		app := appGenerator().Draw(t, "app")
		maxInput, maxOutput := genLimits.Draw(t, "max_input"), genLimits.Draw(t, "max_output")
		l := NewInMemory(maxInput, maxOutput)
		initialConsumptionState := l.Consumption(app)

		_, settleFunc, err := l.Reserve(
			t.Context(),
			ReserveRequest{
				App: app,
				Usage: &Usage{
					InputTokens:  rapid.UintMax(maxInput-1).Draw(t, "reserve_input"),
					OutputTokens: rapid.UintRange(1, maxOutput-1).Draw(t, "reserve_output"),
				},
			},
		)
		require.NoError(t, err)
		require.NotNil(t, settleFunc)

		afterReservation := l.Consumption(app)
		require.GreaterOrEqual(t, afterReservation.InputTokens, initialConsumptionState.InputTokens)
		require.Greater(t, afterReservation.OutputTokens, initialConsumptionState.OutputTokens)

		settleFunc(t.Context(), Usage{})

		require.Equal(t, afterReservation.InputTokens, l.Consumption(app).InputTokens)
		require.Equal(t, afterReservation.OutputTokens, l.Consumption(app).OutputTokens)
	})
}

// TestProperty_ReservationsAndSettlementsCountForLimit given reservations and
// settlements, both contribute to the app limits.
func TestProperty_ReservationsAndSettlementsCountForLimit(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		app := appGenerator().Draw(t, "app")
		maxInput, maxOutput := genLimits.Draw(t, "max_input"), genLimits.Draw(t, "max_output")
		halfInput := (maxInput - 2) / 2
		halfOutput := (maxOutput - 2) / 2
		l := NewInMemory(maxInput, maxOutput)

		var (
			totalInput  uint
			totalOutput uint
		)

		nReservations := rapid.IntRange(1, 10).Draw(t, "n_reservations")
		for _, usage := range usageGenerator(nReservations, halfInput, halfOutput).Draw(t, "reservations") {
			_, _, err := l.Reserve(t.Context(), ReserveRequest{
				App:   app,
				Usage: &usage,
			})
			require.NoError(t, err)
			totalInput += usage.InputTokens
			totalOutput += usage.OutputTokens
		}

		require.Equal(t, totalInput, l.Consumption(app).InputTokens)
		require.Equal(t, totalOutput, l.Consumption(app).OutputTokens)

		nSettled := rapid.IntRange(1, 10).Draw(t, "n_settled")
		for _, usage := range usageGenerator(nSettled, halfInput, halfOutput).Draw(t, "reservations_to_be_settled") {
			_, settleFunc, err := l.Reserve(t.Context(), ReserveRequest{
				App:   app,
				Usage: &usage,
			})
			require.NoError(t, err)
			settleFunc(t.Context(), usage)
			totalInput += usage.InputTokens
			totalOutput += usage.OutputTokens
		}

		require.Equal(t, totalInput, l.Consumption(app).InputTokens)
		require.Equal(t, totalOutput, l.Consumption(app).OutputTokens)

		// Final reservation will top the limits, anything from now on should
		// be rejected.
		//
		// Reserve the settleFunc for later to ensure settling is not affected
		// after the limit is exceeded.
		validUsage := Usage{
			InputTokens:  rapid.Just(maxInput-l.Consumption(app).InputTokens).Draw(t, "valid_reserve_input"),
			OutputTokens: rapid.Just(maxOutput-l.Consumption(app).OutputTokens).Draw(t, "valid_reserve_output"),
		}
		_, settleFunc, err := l.Reserve(t.Context(), ReserveRequest{
			App:   app,
			Usage: &validUsage,
		})
		require.NoError(t, err)
		require.NotNil(t, settleFunc)

		// Any reserve now should fail.
		_, _, err = l.Reserve(t.Context(), ReserveRequest{
			App: app,
			Usage: &Usage{
				InputTokens:  rapid.Just(uint(1)).Draw(t, "final_reserve_input"),
				OutputTokens: rapid.Just(uint(1)).Draw(t, "final_reserve_output"),
			},
		})
		require.Error(t, err)

		settleFunc(t.Context(), validUsage)
	})
}

// TestProperty_SettlementAdjustReservationValues given a settlement, it adjust
// the limit values based on the reported usage.
func TestProperty_SettlementAdjustReservationValues(t *testing.T) {
	minReservation := func(min uint) func(t *rapid.T, maxInput, maxOutput uint) Usage {
		return func(t *rapid.T, maxInput, maxOutput uint) Usage {
			return Usage{
				InputTokens:  rapid.UintRange(min, maxInput-1).Draw(t, "reserve_input"),
				OutputTokens: rapid.UintRange(min, maxOutput-1).Draw(t, "reserve_output"),
			}
		}
	}

	for name, tc := range map[string]struct {
		reserve            func(t *rapid.T, maxInput uint, maxOutput uint) Usage
		usage              func(t *rapid.T, reservedInput uint, reservedOutput uint) Usage
		expectInputTokens  require.ComparisonAssertionFunc
		expectOutputTokens require.ComparisonAssertionFunc
	}{
		"exact token consumption": {
			reserve: minReservation(1),
			usage: func(t *rapid.T, reservedInput, reservedOutput uint) Usage {
				return Usage{
					InputTokens:  rapid.Just(reservedInput).Draw(t, "settle_input"),
					OutputTokens: rapid.Just(reservedOutput).Draw(t, "settle_output"),
				}
			},
			expectInputTokens:  require.Equal,
			expectOutputTokens: require.Equal,
		},
		"consumed more": {
			reserve: minReservation(1),
			usage: func(t *rapid.T, reservedInput, reservedOutput uint) Usage {
				return Usage{
					InputTokens:  rapid.UintMin(reservedInput+1).Draw(t, "settle_input"),
					OutputTokens: rapid.UintMin(reservedOutput+1).Draw(t, "settle_output"),
				}
			},
			expectInputTokens:  require.Greater,
			expectOutputTokens: require.Greater,
		},
		"consumed less": {
			// Create some room so that usage reports less than the value
			// reserved.
			reserve: minReservation(10),
			usage: func(t *rapid.T, reservedInput, reservedOutput uint) Usage {
				return Usage{
					InputTokens:  rapid.UintRange(1, reservedInput-1).Draw(t, "settle_input"),
					OutputTokens: rapid.UintRange(1, reservedOutput-1).Draw(t, "settle_output"),
				}
			},
			expectInputTokens:  require.Less,
			expectOutputTokens: require.Less,
		},
		"empty settlement usage": {
			reserve: minReservation(1),
			usage: func(t *rapid.T, reservedInput, reservedOutput uint) Usage {
				return Usage{
					InputTokens:  rapid.Just(uint(0)).Draw(t, "settle_input"),
					OutputTokens: rapid.Just(uint(0)).Draw(t, "settle_output"),
				}
			},
			expectInputTokens:  require.Equal,
			expectOutputTokens: require.Equal,
		},
		"zero input reservation": {
			reserve: func(t *rapid.T, maxInput, maxOutput uint) Usage {
				return Usage{
					InputTokens:  rapid.Just(uint(0)).Draw(t, "reserve_input"),
					OutputTokens: rapid.Just(uint(1)).Draw(t, "reserve_output"),
				}
			},
			usage: func(t *rapid.T, reservedInput, reservedOutput uint) Usage {
				return Usage{
					InputTokens:  rapid.UintMin(1).Draw(t, "settle_input"),
					OutputTokens: rapid.Just(uint(1)).Draw(t, "settle_output"),
				}
			},
			expectInputTokens:  require.Greater,
			expectOutputTokens: require.Equal,
		},
	} {
		t.Run(name, rapid.MakeCheck(func(t *rapid.T) {
			app := appGenerator().Draw(t, "app")
			maxInput, maxOutput := genLimits.Draw(t, "max_input"), genLimits.Draw(t, "max_output")
			l := NewInMemory(maxInput, maxOutput)

			reservedUsage := tc.reserve(t, maxInput, maxOutput)
			_, settleFunc, err := l.Reserve(
				t.Context(),
				ReserveRequest{
					App:   app,
					Usage: &reservedUsage,
				},
			)
			require.NoError(t, err)
			require.NotNil(t, settleFunc)

			require.Equal(t, reservedUsage.InputTokens, l.Consumption(app).InputTokens)
			require.Equal(t, reservedUsage.OutputTokens, l.Consumption(app).OutputTokens)

			settleFunc(t.Context(), tc.usage(t, reservedUsage.InputTokens, reservedUsage.OutputTokens))

			tc.expectInputTokens(t, l.Consumption(app).InputTokens, reservedUsage.InputTokens)
			tc.expectOutputTokens(t, l.Consumption(app).OutputTokens, reservedUsage.OutputTokens)
		}))
	}
}

// TestProperty_LargeReservationsDoNotBypassLimits given reservations that
// exceed the internal max value, subsequent requests are still rejected.
func TestProperty_LargeReservationsDoNotBypassLimits(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		app := appGenerator().Draw(t, "app")
		// Use limits that are close to the max unit value.
		maxInput, maxOutput := rapid.UintRange(math.MaxUint-10, math.MaxUint-1).Draw(t, "max_input"), rapid.UintRange(math.MaxUint-10, math.MaxUint-1).Draw(t, "max_output")
		l := NewInMemory(maxInput, maxOutput)

		// Initial reservation will almost consume the entire limits.
		_ = reserveAndSettle(t, l, app, Usage{
			InputTokens:  rapid.UintRange(maxInput-10, maxInput-1).Draw(t, "initial_input"),
			OutputTokens: rapid.UintRange(maxOutput-10, maxOutput-1).Draw(t, "initial_output"),
		})

		initialConsumption := l.Consumption(app)

		// Now requests that will push the value into wrapping.
		_, _, err := l.Reserve(t.Context(), ReserveRequest{
			App: app,
			Usage: &Usage{
				InputTokens:  rapid.UintMin(100).Draw(t, "large_input"),
				OutputTokens: rapid.UintMin(100).Draw(t, "large_output"),
			},
		})
		// Those will exceed the limits.
		require.Error(t, err)

		// Just to ensure the limits are not affected.
		require.Equal(t, initialConsumption.InputTokens, l.Consumption(app).InputTokens)
		require.Equal(t, initialConsumption.OutputTokens, l.Consumption(app).OutputTokens)

	})
}

// TestProperty_LargeSettlementsDoNotBypassLimits given settlements reporting
// usage far above what was reserved, the consumption saturates instead of
// wrapping around and resetting the app limits.
func TestProperty_LargeSettlementsDoNotBypassLimits(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		app := appGenerator().Draw(t, "app")
		// Use limits that are close to the max uint value.
		maxInput, maxOutput := rapid.UintRange(math.MaxUint-10, math.MaxUint).Draw(t, "max_input"), rapid.UintRange(math.MaxUint-10, math.MaxUint).Draw(t, "max_output")
		l := NewInMemory(maxInput, maxOutput)

		// Initial reservation will almost consume the entire limits.
		_ = reserveAndSettle(t, l, app, Usage{
			InputTokens:  maxInput - 10,
			OutputTokens: maxOutput - 10,
		})

		// Small reservation that later reports much more usage than reserved.
		_, settleFunc, err := l.Reserve(t.Context(), ReserveRequest{App: app, Usage: &Usage{
			InputTokens:  rapid.UintRange(1, 10).Draw(t, "reserve_input"),
			OutputTokens: rapid.UintRange(1, 10).Draw(t, "reserve_output"),
		}})
		require.NoError(t, err)
		require.NotNil(t, settleFunc)
		settleFunc(t.Context(), Usage{
			InputTokens:  rapid.UintMin(math.MaxUint/2).Draw(t, "settle_input"),
			OutputTokens: rapid.UintMin(math.MaxUint/2).Draw(t, "settle_output"),
		})

		// The over-reported usage must not wrap the consumption back to a low
		// value.
		require.GreaterOrEqual(t, l.Consumption(app).InputTokens, maxInput)
		require.GreaterOrEqual(t, l.Consumption(app).OutputTokens, maxOutput)

		// With the limits consumed, any reservation is rejected.
		_, _, err = l.Reserve(t.Context(), ReserveRequest{App: app, Usage: &Usage{
			InputTokens:  rapid.Just(uint(1)).Draw(t, "final_reserve_input"),
			OutputTokens: rapid.Just(uint(1)).Draw(t, "final_reserve_output"),
		}})
		require.Error(t, err)
	})
}

// TestProperty_SettlementsShouldOnlyApplyOnce given a settlement, calling it
// multiple times should not affect the limits usage.
func TestProperty_SettlementsShouldOnlyApplyOnce(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		app := appGenerator().Draw(t, "app")
		maxInput, maxOutput := genLimits.Draw(t, "max_input"), genLimits.Draw(t, "max_output")
		l := NewInMemory(maxInput, maxOutput)

		usage := Usage{
			InputTokens:  rapid.UintRange(1, maxInput-1).Draw(t, "reserve_input"),
			OutputTokens: rapid.UintRange(1, maxOutput-1).Draw(t, "reserve_output"),
		}
		_, settleFunc, err := l.Reserve(t.Context(), ReserveRequest{App: app, Usage: &usage})
		require.NoError(t, err)
		require.NotNil(t, settleFunc)

		consumption := l.Consumption(app)

		for range rapid.IntRange(2, 5).Draw(t, "n_settlements") {
			settleFunc(t.Context(), usage)
			require.Equal(t, consumption.InputTokens, l.Consumption(app).InputTokens)
			require.Equal(t, consumption.OutputTokens, l.Consumption(app).OutputTokens)
		}
	})
}

var genLimits = rapid.Custom(func(t *rapid.T) uint {
	return rapid.UintMin(32).Draw(t, "")
})

// usageGenerator returns a [Usage] generator that is able to generate a list
// of usage that sum to the inputTotal and outputTotal.
func usageGenerator(n int, inputTotal uint, outputTotal uint) *rapid.Generator[[]Usage] {
	if n == 1 {
		return rapid.Just([]Usage{{
			InputTokens:  inputTotal,
			OutputTokens: outputTotal,
		}})
	}

	return rapid.Custom(func(t *rapid.T) []Usage {
		inputs := drawParts(t, n, inputTotal, "input")
		outputs := drawParts(t, n, outputTotal, "output")

		usages := make([]Usage, n)
		for i := range usages {
			usages[i] = Usage{
				InputTokens:  inputs[i],
				OutputTokens: outputs[i],
			}
		}

		return usages
	})
}

// appGenerator returns a [types.Application] generator.
func appGenerator() *rapid.Generator[types.Application] {
	return rapid.Custom(func(t *rapid.T) types.Application {
		app, err := types.NewAppV3(types.Metadata{
			Name: rapid.StringOfN(rapid.RuneFrom(nil, unicode.ASCII_Hex_Digit), 6, 253, -1).Draw(t, "name"),
		}, types.AppSpecV3{
			LLM: &types.LLM{
				Format: rapid.OneOf(
					rapid.Just(types.LLMFormatAnthropic),
					rapid.Just(types.LLMFormatOpenAI),
				).Draw(t, "llm_format"),
				Provider: types.LLMProviderAWSBedrock,
			},
		})
		require.NoError(t, err)
		return app
	})
}

func reserveAndSettle(t *rapid.T, l *InMemory, app types.Application, usage Usage) ReserveInfo {
	t.Helper()

	info, settleFunc, err := l.Reserve(t.Context(), ReserveRequest{App: app, Usage: &usage})
	require.NoError(t, err)
	require.NotNil(t, settleFunc)
	settleFunc(t.Context(), usage)
	return info
}

func drawParts(t *rapid.T, n int, total uint, label string) []uint {
	t.Helper()
	if n <= 0 || total < uint(n) {
		t.Fatalf("total must be at least n, and n must be positive")
	}

	parts := make([]uint, n)
	remaining := total - uint(n)

	for i := range n - 1 {
		part := rapid.UintRange(0, remaining).Draw(t, fmt.Sprintf("%s-%d", label, i))
		parts[i] = 1 + part
		remaining -= part
	}

	parts[n-1] = 1 + remaining
	return parts
}
