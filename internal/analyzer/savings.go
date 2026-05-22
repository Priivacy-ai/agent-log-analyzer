package analyzer

func estimatePluginSavings(metrics Metrics, findings []Finding, signals AnalysisSignals) SavingsEstimate {
	total := savingsTotalTokens(metrics, signals)
	if total <= 0 || len(findings) == 0 {
		return SavingsEstimate{FindingEstimates: []FindingSavingsEstimate{}}
	}
	out := SavingsEstimate{FindingEstimates: make([]FindingSavingsEstimate, 0, len(findings))}
	for _, finding := range findings {
		gross := grossFindingTokens(finding, metrics, signals, total)
		if gross <= 0 {
			continue
		}
		lowPct, highPct := pluginCaptureRange(finding)
		low := gross * lowPct / 100
		high := gross * highPct / 100
		if high < low {
			low, high = high, low
		}
		if high == 0 {
			high = 1
		}
		out.PotentialTokensLow += low
		out.PotentialTokensHigh += high
		out.FindingEstimates = append(out.FindingEstimates, FindingSavingsEstimate{
			FindingID:           finding.ID,
			GrossTokens:         gross,
			CapturePctLow:       lowPct,
			CapturePctHigh:      highPct,
			PotentialTokensLow:  low,
			PotentialTokensHigh: high,
		})
	}
	capSavingsEstimate(&out, total)
	out.PotentialPctLow = percentOf(out.PotentialTokensLow, total)
	out.PotentialPctHigh = percentOf(out.PotentialTokensHigh, total)
	if out.PotentialPctLow > out.PotentialPctHigh {
		out.PotentialPctLow, out.PotentialPctHigh = out.PotentialPctHigh, out.PotentialPctLow
	}
	return out
}

func savingsTotalTokens(metrics Metrics, signals AnalysisSignals) int {
	total := metrics.EstimatedTokens
	if total <= 0 {
		total = signals.InputTokens + signals.OutputTokens
	}
	if total <= 0 {
		total = metrics.ToolOutputTokens
	}
	return total
}

func grossFindingTokens(finding Finding, metrics Metrics, signals AnalysisSignals, total int) int {
	if total <= 0 {
		return 0
	}
	if finding.Evidence.TokenShare > 0 {
		return clampSavingsTokens(total*finding.Evidence.TokenShare/100, total)
	}
	switch finding.ID {
	case "tool_output_bloat":
		return clampSavingsTokens(metrics.ToolOutputTokens, total)
	case "cache_invalidation_spike":
		if signals.CacheCreationTokens > 0 {
			return clampSavingsTokens(signals.CacheCreationTokens, total)
		}
		return percentageSavingsTokens(total, finding.Evidence.Count, 8, 30)
	case "args_hashed_retry_loop":
		return percentageSavingsTokens(total, finding.Evidence.Count, 6, 30)
	case "retry_loop":
		count := finding.Evidence.Count
		if count == 0 {
			count = metrics.RetryDepthMax
		}
		return percentageSavingsTokens(total, count, 5, 30)
	case "repeated_file_reads":
		count := finding.Evidence.Count
		if count == 0 {
			count = metrics.Rereads
		}
		return percentageSavingsTokens(total, count, 2, 38)
	case "context_growth_spikes":
		count := finding.Evidence.Count
		if count == 0 {
			count = metrics.ContextGrowthEvents
		}
		return percentageSavingsTokens(total, count, 6, 42)
	case "mcp_bloat_severe", "skill_bloat_severe":
		return total * 22 / 100
	case "mcp_bloat_high", "skill_bloat_high":
		return total * 14 / 100
	default:
		return total * 10 / 100
	}
}

func pluginCaptureRange(finding Finding) (int, int) {
	switch finding.ID {
	case "repeated_file_reads":
		return 35, 60
	case "retry_loop", "args_hashed_retry_loop":
		return 25, 50
	case "tool_output_bloat":
		return 20, 45
	case "context_growth_spikes":
		return 15, 35
	case "cache_invalidation_spike":
		return 10, 30
	case "mcp_bloat_severe", "mcp_bloat_high", "skill_bloat_severe", "skill_bloat_high":
		return 10, 25
	default:
		return 10, 20
	}
}

func percentageSavingsTokens(total, count, perCountPct, maxPct int) int {
	if count <= 0 {
		count = 1
	}
	pct := count * perCountPct
	if pct > maxPct {
		pct = maxPct
	}
	if pct < 3 {
		pct = 3
	}
	return clampSavingsTokens(total*pct/100, total)
}

func clampSavingsTokens(tokens, total int) int {
	if tokens < 0 {
		return 0
	}
	if total <= 0 {
		return tokens
	}
	if tokens > total {
		return total
	}
	return tokens
}

func capSavingsEstimate(estimate *SavingsEstimate, total int) {
	if total <= 0 || estimate.PotentialTokensHigh <= 0 {
		return
	}
	capHigh := total * 65 / 100
	if capHigh < 1 {
		capHigh = 1
	}
	if estimate.PotentialTokensHigh <= capHigh {
		if estimate.PotentialTokensLow > estimate.PotentialTokensHigh {
			estimate.PotentialTokensLow = estimate.PotentialTokensHigh
		}
		return
	}
	originalHigh := estimate.PotentialTokensHigh
	estimate.PotentialTokensHigh = capHigh
	estimate.PotentialTokensLow = estimate.PotentialTokensLow * capHigh / originalHigh
	for i := range estimate.FindingEstimates {
		estimate.FindingEstimates[i].PotentialTokensHigh = estimate.FindingEstimates[i].PotentialTokensHigh * capHigh / originalHigh
		estimate.FindingEstimates[i].PotentialTokensLow = estimate.FindingEstimates[i].PotentialTokensLow * capHigh / originalHigh
	}
}

func percentOf(tokens, total int) int {
	if total <= 0 || tokens <= 0 {
		return 0
	}
	return clampPct((tokens*100 + total/2) / total)
}

func scoreFromSavings(savings SavingsEstimate) int {
	return 100 - clampPct(savings.PotentialPctHigh)
}

func wasteRangeFromSavings(savings SavingsEstimate) WasteRange {
	low := clampPct(savings.PotentialPctLow)
	high := clampPct(savings.PotentialPctHigh)
	if low > high {
		low, high = high, low
	}
	return WasteRange{Low: low, High: high}
}
