<script lang="ts">
	import './layout.css';

	let { children } = $props();
</script>

<svelte:head>
	<title>Verve</title>
</svelte:head>

<div class="min-h-screen">
	{@render children()}
</div>
