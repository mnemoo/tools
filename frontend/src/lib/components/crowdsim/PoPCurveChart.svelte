<script lang="ts">
	import { _ } from '$lib/i18n';

	let {
		popCurve,
		initialBalance = 100
	}: {
		popCurve: number[];
		initialBalance?: number;
	} = $props();

	const width = 600;
	const height = 280;
	const padding = { top: 20, right: 20, bottom: 40, left: 50 };

	const chartWidth = width - padding.left - padding.right;
	const chartHeight = height - padding.top - padding.bottom;

	// Scale functions
	function xScale(i: number): number {
		return padding.left + (i / (popCurve.length - 1)) * chartWidth;
	}

	function yScale(v: number): number {
		return padding.top + (1 - v) * chartHeight;
	}

	// Generate path
	let pathData = $derived(
		popCurve
			.map((v, i) => {
				const x = xScale(i);
				const y = yScale(v);
				return i === 0 ? `M ${x} ${y}` : `L ${x} ${y}`;
			})
			.join(' ')
	);

	// Y-axis ticks
	const yTicks = [0, 0.25, 0.5, 0.75, 1.0];

	// X-axis ticks (every 50 spins or so)
	let xTicks = $derived(() => {
		const ticks: number[] = [];
		const step = Math.ceil(popCurve.length / 6);
		for (let i = 0; i < popCurve.length; i += step) {
			ticks.push(i);
		}
		if (ticks[ticks.length - 1] !== popCurve.length - 1) {
			ticks.push(popCurve.length - 1);
		}
		return ticks;
	});

	// Final PoP value
	let finalPoP = $derived(popCurve[popCurve.length - 1] || 0);

	function getPoPColor(pop: number): string {
		if (pop >= 0.4) return 'var(--color-emerald)';
		if (pop >= 0.25) return 'var(--color-gold)';
		return 'var(--color-coral)';
	}
</script>

<div class="rounded-2xl bg-[var(--color-graphite)]/50 border border-white/[0.03] p-5">
	<div class="flex items-center gap-3 mb-4">
		<div class="w-1 h-5 bg-[var(--color-violet)] rounded-full"></div>
		<h3 class="font-mono text-sm text-[var(--color-light)]">{$_('crowdsim.probabilityOfProfit')}</h3>
		<span class="ml-auto text-xs font-mono text-[var(--color-mist)]">{$_('crowdsim.overTime')}</span>
	</div>

	<svg viewBox="0 0 {width} {height}" class="w-full">
		<defs>
			<!-- Gradient for the area fill -->
			<linearGradient id="popGradient" x1="0%" y1="0%" x2="0%" y2="100%">
				<stop offset="0%" style="stop-color: var(--color-violet); stop-opacity: 0.3" />
				<stop offset="100%" style="stop-color: var(--color-violet); stop-opacity: 0" />
			</linearGradient>
			<!-- Glow filter -->
			<filter id="popGlow" x="-20%" y="-20%" width="140%" height="140%">
				<feGaussianBlur stdDeviation="2" result="coloredBlur"/>
				<feMerge>
					<feMergeNode in="coloredBlur"/>
					<feMergeNode in="SourceGraphic"/>
				</feMerge>
			</filter>
		</defs>

		<!-- Grid lines -->
		{#each yTicks as tick}
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

		<!-- 50% reference line (break-even marker) -->
		<line
			x1={padding.left}
			y1={yScale(0.5)}
			x2={width - padding.right}
			y2={yScale(0.5)}
			stroke="var(--color-gold)"
			stroke-width="1.5"
			stroke-dasharray="8,4"
			stroke-opacity="0.6"
		/>

		<!-- Area under curve -->
		<path
			d="{pathData} L {xScale(popCurve.length - 1)} {yScale(0)} L {xScale(0)} {yScale(0)} Z"
			fill="url(#popGradient)"
		/>

		<!-- PoP Curve -->
		<path
			d={pathData}
			fill="none"
			stroke="var(--color-violet)"
			stroke-width="2.5"
			filter="url(#popGlow)"
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
		{#each yTicks as tick}
			<text
				x={padding.left - 8}
				y={yScale(tick)}
				text-anchor="end"
				dominant-baseline="middle"
				class="fill-[var(--color-mist)] text-xs font-mono"
			>
				{(tick * 100).toFixed(0)}%
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

		<!-- X-axis labels (tick is array index, display as spin number = index + 1) -->
		{#each xTicks() as tick}
			<text
				x={xScale(tick)}
				y={height - padding.bottom + 18}
				text-anchor="middle"
				class="fill-[var(--color-mist)] text-xs font-mono"
			>
				{tick + 1}
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

		<!-- Final value marker -->
		<circle
			cx={xScale(popCurve.length - 1)}
			cy={yScale(finalPoP)}
			r="5"
			fill={getPoPColor(finalPoP)}
			filter="url(#popGlow)"
		/>
		<text
			x={xScale(popCurve.length - 1) - 10}
			y={yScale(finalPoP) - 12}
			text-anchor="end"
			class="text-xs font-mono font-semibold"
			fill={getPoPColor(finalPoP)}
		>
			{(finalPoP * 100).toFixed(1)}%
		</text>
	</svg>

	<!-- Legend -->
	<div class="mt-4 flex items-center justify-center gap-6 text-xs font-mono text-[var(--color-mist)]">
		<div class="flex items-center gap-2">
			<div class="h-0.5 w-5 rounded bg-[var(--color-violet)]"></div>
			<span>{$_('crowdsim.popCurve')}</span>
		</div>
		<div class="flex items-center gap-2">
			<div class="h-0.5 w-5 rounded bg-[var(--color-gold)] opacity-60" style="background: repeating-linear-gradient(90deg, var(--color-gold) 0, var(--color-gold) 4px, transparent 4px, transparent 8px);"></div>
			<span>{$_('crowdsim.breakeven50')}</span>
		</div>
	</div>
</div>
