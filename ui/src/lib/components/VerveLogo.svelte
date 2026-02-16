<script lang="ts">
	let { size = 32 }: { size?: number } = $props();
</script>

<svg
	width={size}
	height={size}
	viewBox="0 0 32 32"
	fill="none"
	xmlns="http://www.w3.org/2000/svg"
	aria-hidden="true"
>
	<defs>
		<linearGradient id="verve-bg" x1="0" y1="0" x2="32" y2="32" gradientUnits="userSpaceOnUse">
			<stop offset="0%" stop-color="#7c3aed" />
			<stop offset="100%" stop-color="#a855f7" />
		</linearGradient>
		<linearGradient id="verve-accent" x1="8" y1="8" x2="24" y2="26" gradientUnits="userSpaceOnUse">
			<stop offset="0%" stop-color="#e9d5ff" />
			<stop offset="100%" stop-color="#ffffff" />
		</linearGradient>
		<filter id="verve-glow">
			<feGaussianBlur stdDeviation="0.8" result="blur" />
			<feMerge>
				<feMergeNode in="blur" />
				<feMergeNode in="SourceGraphic" />
			</feMerge>
		</filter>
	</defs>

	<!-- Background with rounded corners -->
	<rect width="32" height="32" rx="7" fill="url(#verve-bg)" />

	<!-- Subtle inner highlight -->
	<rect
		x="0.5"
		y="0.5"
		width="31"
		height="31"
		rx="6.5"
		fill="none"
		stroke="white"
		stroke-opacity="0.15"
	/>

	<!-- Abstract V mark with flowing energy lines -->
	<g filter="url(#verve-glow)">
		<!-- Main V shape — bold, slightly stylized -->
		<path
			d="M8.5 8.5L16 24L23.5 8.5"
			stroke="url(#verve-accent)"
			stroke-width="3"
			stroke-linecap="round"
			stroke-linejoin="round"
			fill="none"
		/>

		<!-- Energy arc — sweeping curve suggesting motion/orchestration -->
		<path
			d="M11 13.5C14.5 15.5 17.5 15.5 21 13.5"
			stroke="white"
			stroke-opacity="0.5"
			stroke-width="1.5"
			stroke-linecap="round"
			fill="none"
		/>

		<!-- Small energy dot at the apex -->
		<circle cx="16" cy="24" r="1.5" fill="white" fill-opacity="0.9" />
	</g>
</svg>
