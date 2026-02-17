<template>
  <v-app>
    <v-app-bar density="compact" color="surface" flat border>
      <template v-slot:prepend>
        <div class="d-flex align-center ml-2">
          <div :style="{ width: '10px', height: '10px', borderRadius: '50%', background: wsConnected ? '#4CAF50' : '#F44336' }" />
        </div>
      </template>

      <v-app-bar-title class="text-body-2 font-weight-bold">Lego ACAP</v-app-bar-title>

      <template v-slot:append>
        <div class="d-flex align-center ga-2">
          <v-progress-circular v-if="running" size="16" width="2" indeterminate color="warning" />

          <!-- Certificate info -->
          <template v-if="certInfo?.has_cert">
            <v-chip size="x-small" color="success" variant="tonal" @click="showCertDialog = true" style="cursor: pointer;">
              <v-icon start size="14">mdi-certificate</v-icon>
              {{ certInfo.domain }}
            </v-chip>
          </template>
          <v-chip v-else size="x-small" color="grey" variant="tonal">
            No certificate
          </v-chip>

          <v-divider vertical class="mx-1" />

          <v-btn variant="text" size="small" :color="legoReady ? 'success' : 'warning'" @click="downloadLego" :loading="downloading" prepend-icon="mdi-download">
            {{ legoReady ? 'Binary ready' : 'Download' }}
          </v-btn>
          <span v-if="downloading" class="text-caption text-grey-lighten-1">{{ downloadPercent }}%</span>

          <v-divider vertical class="mx-1" />

          <v-btn variant="text" size="small" :disabled="!legoReady || running" @click="obtainCert" prepend-icon="mdi-certificate" color="success">
            Obtain
          </v-btn>
          <v-btn variant="text" size="small" :disabled="!legoReady || running || !certInfo?.has_cert" @click="renewCert" prepend-icon="mdi-refresh" color="secondary">
            Renew
          </v-btn>
          <v-btn v-if="running" variant="text" size="small" @click="stopLego" prepend-icon="mdi-stop-circle" color="error">
            Stop
          </v-btn>
        </div>
      </template>
    </v-app-bar>

    <v-main>
      <v-container fluid class="pa-2">
        <v-row dense>
          <!-- LEFT: Config -->
          <v-col cols="12" md="5">
            <v-card class="mb-2">
              <v-card-title class="text-body-2 font-weight-bold py-2">Configuration</v-card-title>
              <v-card-text class="pt-0">
                <div class="field-group">
                  <label class="field-label">Email <span class="text-error">*</span></label>
                  <v-text-field v-model="config.email" density="compact" :rules="[rules.required, rules.email]" hint="ACME account email for certificate notifications" persistent-hint />
                </div>

                <div class="field-group">
                  <label class="field-label">Domains <span class="text-error">*</span></label>
                  <v-text-field v-model="config.domains" density="compact" :rules="[rules.required]" hint="Comma-separated, e.g. example.com, *.example.com" persistent-hint placeholder="example.com, *.example.com" />
                </div>

                <v-row dense>
                  <v-col cols="7">
                    <div class="field-group">
                      <label class="field-label">DNS Provider <span class="text-error">*</span></label>
                      <v-autocomplete v-model="config.dns_provider" :items="dnsProviders" density="compact" :rules="[rules.required]" hint="DNS-01 challenge provider" persistent-hint :placeholder="dnsProviders.length ? 'Select provider...' : 'Type provider name...'" />
                    </div>
                  </v-col>
                  <v-col cols="5">
                    <div class="field-group">
                      <label class="field-label">DNS Resolvers</label>
                      <v-text-field v-model="config.dns_resolvers" density="compact" :rules="[rules.dnsResolvers]" hint="e.g. 8.8.8.8:53" persistent-hint />
                    </div>
                  </v-col>
                </v-row>

                <div class="field-group">
                  <label class="field-label">CA Server</label>
                  <v-text-field v-model="config.ca_server" density="compact" :rules="[rules.url]" hint="ACME directory URL. Default: Let's Encrypt production" persistent-hint />
                </div>

                <div class="field-group">
                  <label class="field-label">Key Type</label>
                  <v-chip-group v-model="config.key_type" mandatory selected-class="text-primary" column>
                    <v-chip v-for="kt in keyTypes" :key="kt" :value="kt" size="small" variant="outlined" filter>
                      <div style="margin-top: 1px;">{{ kt }}</div>
                    </v-chip>
                  </v-chip-group>
                </div>

                <v-divider class="my-3" />

                <div class="d-flex align-center mb-2">
                  <v-switch v-model="config.eab_enabled" label="External Account Binding (EAB)" density="compact" color="primary" hide-details />
                </div>
                <v-row v-if="config.eab_enabled" dense>
                  <v-col cols="5">
                    <div class="field-group">
                      <label class="field-label">EAB Key ID <span class="text-error">*</span></label>
                      <v-text-field v-model="config.eab_kid" density="compact" :rules="[rules.required]" hint="Key identifier from External CA" persistent-hint />
                    </div>
                  </v-col>
                  <v-col cols="7">
                    <div class="field-group">
                      <label class="field-label">EAB HMAC <span class="text-error">*</span></label>
                      <v-text-field v-model="config.eab_hmac" density="compact" :rules="[rules.required]" hint="Base64 URL MAC key" persistent-hint :type="showSecrets ? 'text' : 'password'" />
                    </div>
                  </v-col>
                </v-row>

                <v-divider class="my-3" />

                <div class="text-caption font-weight-bold mb-1">Provider Environment Variables</div>
                <div v-for="(_, key) in envVars" :key="key" class="d-flex ga-1 mb-1 align-center">
                  <v-text-field :model-value="key" density="compact" hide-details readonly style="max-width: 40%;" />
                  <v-text-field v-model="envVars[key]" density="compact" hide-details :type="showSecrets ? 'text' : 'password'" />
                  <v-btn icon size="x-small" color="error" variant="text" @click="removeEnvVar(key)">
                    <v-icon size="16">mdi-delete</v-icon>
                  </v-btn>
                </div>
                <div class="d-flex ga-1 mb-1 align-center">
                  <v-text-field v-model="newEnvKey" placeholder="Key" density="compact" hide-details style="max-width: 40%;" />
                  <v-text-field v-model="newEnvValue" placeholder="Value" density="compact" hide-details />
                  <v-btn icon size="x-small" color="success" variant="text" @click="addEnvVar" :disabled="!newEnvKey">
                    <v-icon size="16">mdi-plus</v-icon>
                  </v-btn>
                </div>
                <div class="d-flex align-center justify-space-between mt-2">
                  <v-switch v-model="showSecrets" label="Show values" density="compact" color="primary" hide-details />
                  <v-btn color="primary" variant="flat" size="small" @click="saveConfig" :loading="saving" :disabled="!isConfigValid">Save</v-btn>
                </div>
              </v-card-text>
            </v-card>
          </v-col>

          <!-- RIGHT: Log -->
          <v-col cols="12" md="7">
            <v-card>
              <v-card-title class="d-flex align-center text-body-2 font-weight-bold py-2">
                Log Output
                <v-spacer />
                <v-chip v-if="running" color="warning" size="x-small" variant="flat" class="mr-2">Running</v-chip>
                <v-btn size="x-small" variant="text" @click="logLines = []">Clear</v-btn>
              </v-card-title>
              <v-card-text class="pt-0">
                <div ref="logContainer" class="pa-2 rounded" style="height: calc(100vh - 100px); overflow-y: auto; background: #0d0d0d; font-family: monospace; font-size: 12px; line-height: 1.4;">
                  <div v-if="logLines.length === 0" class="text-grey">No output yet...</div>
                  <div v-for="(line, i) in logLines" :key="i" :class="line.startsWith('ERROR') ? 'text-error' : 'text-grey-lighten-2'">{{ line }}</div>
                </div>
              </v-card-text>
            </v-card>
          </v-col>
        </v-row>
      </v-container>
    </v-main>

    <!-- Certificate details dialog -->
    <v-dialog v-model="showCertDialog" max-width="520">
      <v-card>
        <v-card-title class="d-flex align-center text-body-1 font-weight-bold">
          <v-icon class="mr-2" color="success">mdi-certificate</v-icon>
          Certificate Details
        </v-card-title>
        <v-card-text v-if="certInfo?.has_cert">
          <v-table density="compact">
            <tbody>
              <tr>
                <td class="font-weight-bold text-no-wrap">Domain</td>
                <td>{{ certInfo.domain }}</td>
              </tr>
              <tr v-if="certInfo.issuer">
                <td class="font-weight-bold text-no-wrap">Issuer</td>
                <td>{{ certInfo.issuer }}</td>
              </tr>
              <tr v-if="certInfo.not_before">
                <td class="font-weight-bold text-no-wrap">Valid From</td>
                <td>{{ certInfo.not_before }}</td>
              </tr>
              <tr v-if="certInfo.not_after">
                <td class="font-weight-bold text-no-wrap">Valid Until</td>
                <td>{{ certInfo.not_after }}</td>
              </tr>
              <tr v-if="certInfo.san?.length">
                <td class="font-weight-bold text-no-wrap">SAN</td>
                <td>{{ certInfo.san.join(', ') }}</td>
              </tr>
              <tr v-if="certInfo.serial">
                <td class="font-weight-bold text-no-wrap">Serial</td>
                <td class="text-caption" style="word-break: break-all;">{{ certInfo.serial }}</td>
              </tr>
            </tbody>
          </v-table>
        </v-card-text>
        <v-card-actions>
          <v-btn variant="tonal" size="small" color="primary" prepend-icon="mdi-download" @click="downloadCertFile('crt')">Certificate</v-btn>
          <v-btn variant="tonal" size="small" color="primary" prepend-icon="mdi-download" @click="downloadCertFile('key')">Private Key</v-btn>
          <v-btn variant="tonal" size="small" prepend-icon="mdi-download" @click="downloadCertFile('issuer')">Issuer Cert</v-btn>
          <v-spacer />
          <v-btn variant="text" size="small" @click="showCertDialog = false">Close</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-snackbar v-model="snackbar" :color="snackbarColor" :timeout="3000">
      {{ snackbarText }}
    </v-snackbar>
  </v-app>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, nextTick } from 'vue'

const isDev = import.meta.env.DEV
const baseUrl = isDev ? '/api' : './api'
const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
const wsUrl = isDev
  ? `ws://${window.location.host}/ws`
  : `${wsProtocol}//${window.location.host}${window.location.pathname.replace(/\/[^/]*$/, '')}/../legoacap_ws/ws`

const wsConnected = ref(false)
const legoReady = ref(false)
const arch = ref('')
const downloading = ref(false)
const downloadPercent = ref(0)
const downloadMessage = ref('')
const running = ref(false)
const saving = ref(false)
const showSecrets = ref(false)
const showCertDialog = ref(false)
const logLines = ref<string[]>([])
const logContainer = ref<HTMLElement | null>(null)

const config = reactive({
  email: '',
  domains: '',
  dns_provider: '',
  dns_resolvers: '8.8.8.8:53',
  ca_server: 'https://acme-v02.api.letsencrypt.org/directory',
  key_type: 'ec256',
  env_vars: '{}',
  eab_enabled: false,
  eab_kid: '',
  eab_hmac: '',
})

const envVars = reactive<Record<string, string>>({})
const newEnvKey = ref('')
const newEnvValue = ref('')
const certInfo = ref<{
  has_cert: boolean
  domain: string
  issuer?: string
  not_before?: string
  not_after?: string
  san?: string[]
  serial?: string
} | null>(null)
const dnsProviders = ref<string[]>([])
const keyTypes = ['ec256', 'ec384', 'rsa2048', 'rsa4096']

const rules = {
  required: (v: string) => !!v || 'Required',
  email: (v: string) => /.+@.+\..+/.test(v) || 'Invalid email',
  url: (v: string) => !v || /^https?:\/\/.+/.test(v) || 'Must be a valid URL (https://...)',
  dnsResolvers: (v: string) => !v || /^[\w.:, ]+$/.test(v) || 'Invalid format, e.g. 8.8.8.8:53',
}

const isConfigValid = computed(() =>
  !!config.email && /.+@.+\..+/.test(config.email) &&
  !!config.domains &&
  !!config.dns_provider &&
  (!config.ca_server || /^https?:\/\/.+/.test(config.ca_server)) &&
  (!config.dns_resolvers || /^[\w.:, ]+$/.test(config.dns_resolvers)) &&
  (!config.eab_enabled || (!!config.eab_kid && !!config.eab_hmac))
)

const snackbar = ref(false)
const snackbarText = ref('')
const snackbarColor = ref('success')

function showMessage(text: string, color = 'success') {
  snackbarText.value = text
  snackbarColor.value = color
  snackbar.value = true
}

function addEnvVar() {
  if (newEnvKey.value) {
    envVars[newEnvKey.value] = newEnvValue.value
    newEnvKey.value = ''
    newEnvValue.value = ''
  }
}

function removeEnvVar(key: string) {
  delete envVars[key]
}

function syncEnvVarsFromConfig() {
  try {
    const parsed = JSON.parse(config.env_vars || '{}')
    Object.keys(envVars).forEach(k => delete envVars[k])
    Object.assign(envVars, parsed)
  } catch { /* ignore */ }
}

function syncEnvVarsToConfig() {
  config.env_vars = JSON.stringify(envVars)
}

async function fetchStatus() {
  try {
    const res = await fetch(`${baseUrl}/status`)
    const data = await res.json()
    legoReady.value = data.lego_ready
    running.value = data.lego_running
    arch.value = data.arch
  } catch { /* ignore */ }
}

async function fetchConfig() {
  try {
    const res = await fetch(`${baseUrl}/config`)
    if (res.ok) {
      const data = await res.json()
      Object.assign(config, data)
      syncEnvVarsFromConfig()
    }
  } catch { /* ignore */ }
}

async function fetchProviders() {
  try {
    const res = await fetch(`${baseUrl}/providers`)
    if (res.ok) {
      dnsProviders.value = await res.json()
    }
  } catch { /* ignore */ }
}

async function fetchCertInfo() {
  try {
    const res = await fetch(`${baseUrl}/cert`)
    if (res.ok) {
      certInfo.value = await res.json()
    }
  } catch { /* ignore */ }
}

async function saveConfig() {
  saving.value = true
  syncEnvVarsToConfig()
  try {
    const res = await fetch(`${baseUrl}/config`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(config),
    })
    if (res.ok) {
      showMessage('Configuration saved')
    } else {
      showMessage('Failed to save', 'error')
    }
  } catch {
    showMessage('Failed to save', 'error')
  } finally {
    saving.value = false
  }
}

async function downloadLego() {
  downloading.value = true
  downloadPercent.value = 0
  downloadMessage.value = 'Starting download...'
  try {
    await fetch(`${baseUrl}/download`, { method: 'POST' })
  } catch {
    showMessage('Failed to start download', 'error')
    downloading.value = false
  }
}

async function obtainCert() {
  running.value = true
  logLines.value.push('--- Starting certificate obtain ---')
  try {
    await fetch(`${baseUrl}/obtain`, { method: 'POST' })
  } catch {
    showMessage('Failed to start obtain', 'error')
    running.value = false
  }
}

async function renewCert() {
  running.value = true
  logLines.value.push('--- Starting certificate renewal ---')
  try {
    await fetch(`${baseUrl}/renew`, { method: 'POST' })
  } catch {
    showMessage('Failed to start renewal', 'error')
    running.value = false
  }
}

async function stopLego() {
  try {
    const res = await fetch(`${baseUrl}/stop`, { method: 'POST' })
    if (res.ok) {
      running.value = false
      showMessage('Lego process stopped')
    } else {
      const data = await res.json()
      showMessage(data.error || 'Failed to stop', 'error')
    }
  } catch {
    showMessage('Failed to stop', 'error')
  }
}

function downloadCertFile(type: string) {
  window.open(`${baseUrl}/cert/download/${type}`, '_blank')
}

function appendLog(line: string) {
  logLines.value.push(line)
  nextTick(() => {
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  })
}

let ws: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null

function connectWebSocket() {
  ws = new WebSocket(wsUrl)

  ws.onopen = () => {
    wsConnected.value = true
    fetchStatus()
    fetchCertInfo()
    fetchProviders()
  }

  ws.onclose = () => {
    wsConnected.value = false
    reconnectTimer = setTimeout(connectWebSocket, 3000)
  }

  ws.onerror = () => {
    ws?.close()
  }

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      switch (msg.type) {
        case 'download_progress':
          downloading.value = true
          downloadPercent.value = msg.data.percent || 0
          downloadMessage.value = msg.data.message || ''
          break
        case 'download_complete':
          downloading.value = false
          legoReady.value = true
          downloadPercent.value = 100
          showMessage(msg.data.message || 'Download complete')
          fetchProviders()
          break
        case 'download_error':
          downloading.value = false
          showMessage(msg.data.error || 'Download failed', 'error')
          break
        case 'lego_output':
          appendLog(msg.data.line || '')
          break
        case 'lego_complete':
          running.value = false
          appendLog('--- ' + (msg.data.message || 'Done') + ' ---')
          showMessage(msg.data.message || 'Done')
          fetchCertInfo()
          break
        case 'lego_error':
          running.value = false
          appendLog('ERROR: ' + (msg.data.error || 'Unknown error'))
          showMessage(msg.data.error || 'Lego error', 'error')
          break
      }
    } catch { /* ignore */ }
  }
}

onMounted(() => {
  fetchStatus()
  fetchConfig()
  fetchCertInfo()
  fetchProviders()
  connectWebSocket()
})

onUnmounted(() => {
  if (reconnectTimer) clearTimeout(reconnectTimer)
  ws?.close()
})
</script>

<style>
.v-input .v-input__details {
  padding-inline: 0px !important;
}
</style>

<style scoped>
.field-group {
  margin-bottom: 12px;
}
.field-label {
  display: block;
  font-size: 0.8rem;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.7);
}
</style>
