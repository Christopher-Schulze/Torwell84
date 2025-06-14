<script lang="ts">
import { onMount } from 'svelte';

let connected = false;
let progress = 0;
let showLogs = false;
let showSettings = false;

const countries = [
  "Deutschland","Frankreich","Belgien","Schweiz","Liechtenstein",
  "Luxemburg","\u00D6sterreich","Spanien","Italien","Portugal",
  "Russland","Rum\u00E4nien","T\u00FCrkei","UK","USA","Kanada",
  "Mexiko","Brasilien","Argentinien","Japan","China","Antarktis"
];

let entry = countries[0];
let middle = countries[1];
let exit = countries[2];
interface Worker { URL: string; Active: boolean }
let workers: Worker[] = [];
let connectionLogs: string[] = [];
let systemLogs: string[] = [];
let obfs4 = true;
let prewarm = true;
let newWorker = '';
let status = '';

$: barColor = progress < 40 ? '#9c27b0' : progress < 80 ? '#2196f3' : '#4caf50';

async function fetchStatus() {
  const res = await fetch('/status');
  if (res.ok) {
    const data = await res.json();
    connected = data.connected;
    progress = data.progress;
    status = data.status || '';
    workers = data.workers;
    if (data.config) {
      obfs4 = data.config.obfs4;
      prewarm = data.config.prewarm;
    }
  }
}

onMount(() => {
  fetchStatus();
  const id = setInterval(fetchStatus, 1000);
  if ('serviceWorker' in navigator) {
    navigator.serviceWorker.register('/service-worker.js');
  }
  return () => clearInterval(id);
});

async function connect() {
  await fetch('/connect', { method: 'POST' });
  connected = true;
}

async function disconnect() {
  await fetch('/disconnect', { method: 'POST' });
  connected = false;
}

async function newCircuit() {
  await fetch('/new-circuit', { method: 'POST' });
}

async function newIdentity() {
  await fetch('/new-identity', { method: 'POST' });
}

async function loadLogs() {
  connectionLogs = await fetch('/logs/connection').then((r) => r.json());
  systemLogs = await fetch('/logs/general').then((r) => r.json());
}

async function uploadTorrc(files: FileList | null) {
  if (!files || !files[0]) return;
  const form = new FormData();
  form.append('file', files[0]);
  await fetch('/torrc', { method: 'POST', body: form });
}

async function saveConfig() {
  await fetch('/config', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ obfs4, prewarm })
  });
}

async function addWorker() {
  if (!newWorker) return;
  const res = await fetch('/workers', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ URL: newWorker })
  });
  if (res.ok) {
    newWorker = '';
    await fetchStatus();
  }
}

async function testWorker(url: string) {
  const res = await fetch('/workers/test', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ URL: url })
  });
  if (res.ok) {
    await fetchStatus();
  }
}

async function testAllWorkers() {
  const res = await fetch('/workers/test', { method: 'POST' });
  if (res.ok) {
    await fetchStatus();
  }
}

async function removeWorker(url: string) {
  const res = await fetch('/workers', {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ URL: url })
  });
  if (res.ok) {
    await fetchStatus();
  }
}
</script>

<style>
.progress {
  height: 6px;
  margin-bottom: 10px;
  overflow: hidden;
  border-radius: 3px;
}
.bar {
  height: 100%;
  background: var(--bar-color);
  transition: width 0.3s ease;
  will-change: width, transform;
  transform: translateZ(0);
}
.chain {
  display: flex;
  justify-content: space-around;
  margin: 20px 0;
}
.node {
  text-align: center;
  padding: 10px;
  min-width: 70px;
  transition: transform 0.3s ease;
  will-change: transform;
}
.node.glass:hover {
  transform: scale(1.05);
}
.buttons {
  display: flex;
  gap: 10px;
  margin-top: 20px;
}
.modal {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
}
.modal-content {
  padding: 20px;
  min-width: 300px;
  max-height: 80vh;
  overflow: auto;
}
</style>

<div class="progress glass">
    <div class="bar" style="width:{progress}%; --bar-color:{barColor}"></div>
<div>{status}</div>
</div>

<div class="chain">
  <div class="node glass">
    <div>U</div>
    <div>You</div>
  </div>
  <div class="node glass">
    <select bind:value={entry}>
      {#each countries as c}
        <option>{c}</option>
      {/each}
    </select>
    <div>Entry</div>
  </div>
  <div class="node glass">
    <select bind:value={middle}>
      {#each countries as c}
        <option>{c}</option>
      {/each}
    </select>
    <div>Middle</div>
  </div>
  <div class="node glass">
    <select bind:value={exit}>
      {#each countries as c}
        <option>{c}</option>
      {/each}
    </select>
    <div>Exit</div>
  </div>
  <div class="node glass" style="opacity:{workers.some(w => w.Active) ? 1 : 0.3}">
    <div>CF</div>
    <div>Worker</div>
  </div>
</div>

<div class="buttons">
  {#if connected}
    <button on:click={disconnect}>Disconnect</button>
    <button on:click={newCircuit}>New Circuit</button>
    <button on:click={newIdentity}>New Identity</button>
  {:else}
    <button on:click={connect}>Connect</button>
  {/if}
  <button on:click={() => { loadLogs(); showLogs = true; }}>Logs</button>
  <button on:click={() => (showSettings = true)}>Settings</button>
</div>

{#if showLogs}
  <div class="modal" on:click={() => (showLogs = false)}>
    <div class="modal-content glass" on:click|stopPropagation>
      <h2>Logs</h2>
      <h3>Connection</h3>
      <ul>{#each connectionLogs as l}<li>{l}</li>{/each}</ul>
      <h3>System</h3>
      <ul>{#each systemLogs as l}<li>{l}</li>{/each}</ul>
      <button on:click={() => { connectionLogs = []; systemLogs = []; }}>Clear</button>
      <button on:click={() => (showLogs = false)}>Close</button>
    </div>
  </div>
{/if}

{#if showSettings}
  <div class="modal" on:click={() => (showSettings = false)}>
    <div class="modal-content glass" on:click|stopPropagation>
      <h2>Settings</h2>
      <label><input type="checkbox" bind:checked={obfs4} on:change={saveConfig}> OBFS4</label>
      <label><input type="checkbox" bind:checked={prewarm} on:change={saveConfig}> Circuit Pre-Warm</label>
      <div>
        <label>torrc upload <input type="file" on:change={(e) => uploadTorrc(e.target.files)}></label>
      </div>
      <div>
        <h3>Cloudflare Workers</h3>
        <ul>
          {#each workers as w}
            <li>{w.URL} {#if !w.Active}(inactive){/if}
              <button on:click={() => testWorker(w.URL)}>Test</button>
              <button on:click={() => removeWorker(w.URL)}>Remove</button>
            </li>
          {/each}
        </ul>
        <input bind:value={newWorker} placeholder="https://example.workers.dev" />
        <button on:click={addWorker}>Add</button>
        <button on:click={testAllWorkers}>Test All</button>
      </div>
      <button on:click={() => (showSettings = false)}>Close</button>
    </div>
  </div>
{/if}
