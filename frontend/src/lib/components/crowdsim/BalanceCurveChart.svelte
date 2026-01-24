<script lang="ts">
	import type { CrowdSimBalanceCurvePoint } from '$lib/api/types';
	import { _ } from '$lib/i18n';

	let {
		balanceCurve,
		initialBalance = 100
	}: {
		balanceCurve: CrowdSimBalanceCurvePoint[];
		initialBalance?: number;
	} = $props();

	const width = 600;
	const height = 280;
	const padding = { top: 20, right: 20, bottom: 40, left: 60 };

	const chartWidth = width - padding.left - padding.right;
	const chartHeight = height - padding.top - padding.bottom;

	// Find min/max values for Y scale
	let yMin = $derived(Math.min(...balanceCurve.map((p) => p.p5)) * 0.95);
	let yMax = $derived(Math.max(...balanceCurve.map((p) => p.p95)) * 1.05);
	let maxSpin = $derived(balanceCurve[balanceCurve.length - 1]?.spin || 100);

	// Scale functions
	function xScale(spin: number): number {
		return padding.left + (spin / maxSpin) * chartWidth;
	}

	function yScale(v: number): number {
		return padding.top + ((yMax - v) / (yMax - yMin)) * chartHeight;
	}

	// Generate paths for each line
	function generatePath(data: CrowdSimBalanceCurvePoint[], accessor: (p: CrowdSimBalanceCurvePoint) => number): string {
		return data
			.map((p, i) => {
				const x = xScale(p.spin);
				const y = yScale(accessor(p));
				return i === 0 ? `M ${x} ${y}` : `L ${x} ${y}`;
			})
			.join(' ');
	}

	let avgPath = $derived(generatePath(balanceCurve, (p) => p.avg));
	let medianPath = $derived(generatePath(balanceCurve, (p) => p.median));

	// Area between p5 and p95 (confidence band)
	let bandPath = $derived(() => {
		const topLine = balanceCurve.map((p) => `${xScale(p.spin)},${yScale(p.p95)}`).join(' L ');
		const bottomLine = balanceCurve
			.slice()
			.reverse()
			.map((p) => `${xScale(p.spin)},${yScale(p.p5)}`)
			.join(' L ');
		return `M ${topLine} L ${bottomLine} Z`;
	});

	// Y-axis ticks
	let yTicks = $derived(() => {
		const range = yMax - yMin;
		const step = range / 5;
		const ticks: number[] = [];
		for (let i = 0; i <= 5; i++) {
			ticks.push(yMin + step * i);
		}
		return ticks;
	});

	// X-axis ticks
	let xTicks = $derived(() => {
		const ticks: number[] = [];
		const step = Math.ceil(maxSpin / 6);
		for (let i = 0; i <= maxSpin; i += step) {
			ticks.push(i);
		}
		if (ticks[ticks.length - 1] !== maxSpin) {
			ticks.push(maxSpin);
		}
		return ticks;
	});

	// Final values
	let finalPoint = $derived(balanceCurve[balanceCurve.length - 1]);

	// Format number for display
	function formatBalance(v: number): string {
		if (v >= 1000) return (v / 1000).toFixed(1) + 'k';
		return v.toFixed(0);
	}
</script>

<div class="rounded-2xl bg-[var(--color-graphite)]/50 border border-white/[0.03] p-5">
	<div class="flex items-center gap-3 mb-4">
		<div class="w-1 h-5 bg-[var(--color-cyan)] rounded-full"></div>
		<h3 class="font-mono text-sm text-[var(--color-light)]">{$_('crowdsim.avgBalance')}</h3>
		<span class="ml-auto text-xs font-mono text-[var(--color-mist)]">{$_('crowdsim.overTime')}</span>
	</div>

	<svg viewBox="0 0 {width} {height}" class="w-full">
		<defs>
			<!-- Gradient for confidence band -->
			<linearGradient id="bandGradient" x1="0%" y1="0%" x2="0%" y2="100%">
				<stop offset="0%" style="stop-color: var(--color-cyan); stop-opacity: 0.15" />
				<stop offset="50%" style="stop-color: var(--color-cyan); stop-opacity: 0.08" />
				<stop offset="100%" style="stop-color: var(--color-cyan); stop-opacity: 0.15" />
			</linearGradient>
			<!-- Glow filter -->
			<filter id="balanceGlow" x="-20%" y="-20%" width="140%" height="140%">
				<feGaussianBlur stdDeviation="2" result="coloredBlur"/>
				<feMerge>
					<feMergeNode in="coloredBlur"/>
					<feMergeNode in="SourceGraphic"/>
				</feMerge>
			</filter>
		</defs>

		<!-- Grid lines -->
		{#each yTicks() as tick}
			<line
				x1={padding.left}
				y1={yScale(tick)}
				x2={width - padding.right}
				y2={yScale(tick)}
				stroke="var(--color-slate)"
				stroke-opacity="0.2"
				stroke-dasharray="4,4"
			/>
		{/each}

		<!-- Initial balance reference line -->
		{#if initialBalance >= yMin && initialBalance <= yMax}
			<line
				x1={padding.left}
				y1={yScale(initialBalance)}
				x2={width - padding.right}
				y2={yScale(initialBalance)}
				stroke="var(--color-gold)"
				stroke-width="1.5"
				stroke-dasharray="8,4"
				stroke-opacity="0.6"
			/>
		{/if}

		<!-- P5-P95 confidence band -->
		<path d={bandPath()} fill="url(#bandGradient)" />

		<!-- Median line -->
		<path
			d={medianPath}
			fill="none"
			stroke="var(--color-cyan)"
			stroke-width="1.5"
			stroke-dasharray="4,2"
			stroke-opacity="0.5"
		/>

		<!-- Average line -->
		<path
			d={avgPath}
			fill="none"
			stroke="var(--color-cyan)"
			stroke-width="2.5"
			filter="url(#balanceGlow)"
		/>

		<!-- Y-axis -->
		<line
			x1={padding.left}
			y1={padding.top}
			x2={padding.left}
			y2={height - padding.bottom}
			stroke="var(--color-slate)"
			stroke-opacity="0.3"
		/>

		<!-- Y-axis labels -->
		{#each yTicks() as tick}
			<text
				x={padding.left - 8}
				y={yScale(tick)}
				text-anchor="end"
				dominant-baseline="middle"
				class="fill-[var(--color-mist)] text-xs font-mono"
			>
				{formatBalance(tick)}
			</text>
		{/each}

		<!-- X-axis -->
		<line
			x1={padding.left}
			y1={height - padding.bottom}
			x2={width - padding.right}
			y2={height - padding.bottom}
			stroke="var(--color-slate)"
			stroke-opacity="0.3"
		/>

		<!-- X-axis labels -->
		{#each xTicks() as tick}
			<text
				x={xScale(tick)}
				y={height - padding.bottom + 18}
				text-anchor="middle"
				class="fill-[var(--color-mist)] text-xs font-mono"
			>
				{tick}
			</text>
		{/each}

		<!-- X-axis title -->
		<text
			x={width / 2}
			y={height - 5}
			text-anchor="middle"
			class="fill-[var(--color-mist)] text-xs font-mono"
		>
			{$_('crowdsim.spinNumber')}
		</text>

		<!-- Final value markers -->
		{#if finalPoint}
			<circle
				cx={xScale(finalPoint.spin)}
				cy={yScale(finalPoint.avg)}
				r="5"
				fill="var(--color-cyan)"
				filter="url(#balanceGlow)"
			/>
			<text
				x={xScale(finalPoint.spin) - 10}
				y={yScale(finalPoint.avg) - 12}
				text-anchor="end"
				class="fill-[var(--color-cyan)] text-xs font-mono font-semibold"
			>
				{formatBalance(finalPoint.avg)}
			</text>
		{/if}
	</svg>

	<!-- Legend -->
	<div class="mt-4 flex flex-wrap items-center justify-center gap-4 text-xs font-mono text-[var(--color-mist)]">
		<div class="flex items-center gap-2">
			<div class="h-0.5 w-5 rounded bg-[var(--color-cyan)]"></div>
			<span>{$_('crowdsim.average')}</span>
		</div>
		<div class="flex items-center gap-2">
			<div class="h-0.5 w-5 rounded bg-[var(--color-cyan)] opacity-50" style="background: repeating-linear-gradient(90deg, var(--color-cyan) 0, var(--color-cyan) 3px, transparent 3px, transparent 6px);"></div>
			<span>{$_('crowdsim.median')}</span>
		</div>
		<div class="flex items-center gap-2">
			<div class="h-3 w-5 rounded bg-[var(--color-cyan)]/15"></div>
			<span>{$_('crowdsim.p5p95')}</span>
		</div>
		<div class="flex items-center gap-2">
			<div class="h-0.5 w-5 rounded bg-[var(--color-gold)] opacity-60" style="background: repeating-linear-gradient(90deg, var(--color-gold) 0, var(--color-gold) 4px, transparent 4px, transparent 8px);"></div>
			<span>{$_('crowdsim.initialValue', { values: { value: initialBalance } })}</span>
		</div>
	</div>
</div>
