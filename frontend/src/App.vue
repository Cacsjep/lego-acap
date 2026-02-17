<template>
  <v-app>
    <v-app-bar density="compact" color="surface" flat border>
      <template v-slot:prepend>
        <div class="d-flex align-center ml-2">
          <div :style="{ width: '10px', height: '10px', borderRadius: '50%', background: wsConnected ? '#4CAF50' : '#F44336' }" />
        </div>
      </template>

      <v-app-bar-title class="text-body-2 font-weight-bold">
        Lego ACAP
        <v-btn icon size="x-small" variant="text" @click="showHelpDialog = true" class="ml-1">
          <v-icon size="16">mdi-help-circle-outline</v-icon>
        </v-btn>
      </v-app-bar-title>

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
          <v-btn v-if="certInfo?.has_cert" variant="text" size="small" :disabled="running || installing" :loading="installing" @click="installCert" prepend-icon="mdi-upload" color="info">
            Install
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
              <v-card-title class="d-flex align-center text-body-2 font-weight-bold py-2" style="position: sticky; top: 0; z-index: 1; background: rgb(var(--v-theme-surface));">
                Configuration
                <v-spacer />
                <v-btn color="primary" variant="flat" size="small" @click="saveConfig" :loading="saving" :disabled="!isConfigValid">Save</v-btn>
              </v-card-title>
              <v-card-text class="pt-0">
                <v-row dense>
                  <v-col cols="5">
                    <div class="field-group">
                      <label class="field-label">Email <span class="text-error">*</span></label>
                      <v-text-field v-model="config.email" density="compact" :rules="[rules.required, rules.email]" hint="ACME account email" persistent-hint />
                    </div>
                  </v-col>
                  <v-col cols="7">
                    <div class="field-group">
                      <label class="field-label">Domains <span class="text-error">*</span></label>
                      <v-text-field v-model="config.domains" density="compact" :rules="[rules.required]" hint="Comma-separated, e.g. example.com, *.example.com" persistent-hint placeholder="example.com, *.example.com" />
                    </div>
                  </v-col>
                </v-row>

                <v-row dense>
                  <v-col cols="5">
                    <div class="field-group">
                      <label class="field-label">DNS Provider <span class="text-error">*</span></label>
                      <v-autocomplete v-model="config.dns_provider" :items="dnsProviders" density="compact" :rules="[rules.required]" hint="DNS-01 challenge provider" persistent-hint :placeholder="dnsProviders.length ? 'Select provider...' : 'Type provider name...'" />
                    </div>
                  </v-col>
                  <v-col cols="7">
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
                    <v-chip v-for="kt in keyTypes" :key="kt" :value="kt" size="small" variant="outlined" filter>{{ kt }}</v-chip>
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
                      <v-text-field v-model="config.eab_hmac" density="compact" :rules="[rules.required]" hint="Base64 URL MAC key" persistent-hint :type="showEabHmac ? 'text' : 'password'" :append-inner-icon="showEabHmac ? 'mdi-eye-off' : 'mdi-eye'" @click:append-inner="showEabHmac = !showEabHmac" />
                    </div>
                  </v-col>
                </v-row>

                <v-divider class="my-3" />

                <div class="text-caption font-weight-bold mb-1">Provider Environment Variables</div>
                <div v-for="(_, key) in envVars" :key="key" class="d-flex ga-1 mb-1 align-center">
                  <v-text-field :model-value="key" density="compact" hide-details readonly style="max-width: 40%;" />
                  <v-text-field v-model="envVars[key]" density="compact" hide-details :type="envVarVisible[key] ? 'text' : 'password'" :append-inner-icon="envVarVisible[key] ? 'mdi-eye-off' : 'mdi-eye'" @click:append-inner="envVarVisible[key] = !envVarVisible[key]" />
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

                <v-divider class="my-3" />

                <div class="text-caption font-weight-bold mb-1">Automation</div>
                <v-row dense>
                  <v-col cols="5">
                    <div class="field-group">
                      <label class="field-label">Auto mode</label>
                      <v-select v-model="config.auto_mode" :items="[{ title: 'Disabled', value: false }, { title: 'Enabled', value: true }]" density="compact" hint="Auto-renew and install every 24h" persistent-hint />
                    </div>
                  </v-col>
                  <v-col cols="7">
                    <div class="field-group">
                      <label class="field-label">Days before expiry</label>
                      <v-text-field v-model.number="config.auto_days" type="number" density="compact" :rules="[rules.nonNegativeInt]" hint="Renew when cert expires within N days" persistent-hint :disabled="!config.auto_mode" />
                    </div>
                  </v-col>
                </v-row>
              </v-card-text>
            </v-card>
          </v-col>

          <!-- RIGHT: Last Run + Log -->
          <v-col cols="12" md="7">
            <v-card v-if="lastRun" class="mb-2">
              <v-card-title class="d-flex align-center text-body-2 font-weight-bold py-2">
                Last Run
                <v-spacer />
                <v-chip size="x-small" :color="lastRun.success ? 'success' : 'error'" variant="flat" class="mr-2">
                  {{ lastRun.success ? 'Success' : 'Failed' }}
                </v-chip>
                <v-chip size="x-small" variant="tonal" class="mr-2">{{ lastRun.command }}</v-chip>
                <v-btn size="x-small" variant="text" @click="showLastRunLog = !showLastRunLog">
                  {{ showLastRunLog ? 'Hide' : 'Show log' }}
                </v-btn>
              </v-card-title>
              <v-card-text class="pt-0 pb-2">
                <div class="text-caption text-grey">{{ new Date(lastRun.created_at).toLocaleString() }}</div>
                <div v-if="showLastRunLog" class="pa-2 mt-2 rounded" style="max-height: 200px; overflow-y: auto; background: #0d0d0d; font-family: monospace; font-size: 12px; line-height: 1.4;">
                  <div v-for="(line, i) in lastRun.output.split('\n').filter((l: string) => l)" :key="i" :class="logLineClass(line)">{{ line }}</div>
                </div>
              </v-card-text>
            </v-card>

            <v-card>
              <v-card-title class="d-flex align-center text-body-2 font-weight-bold py-2">
                Lego Log Output
                <v-spacer />
                <v-chip v-if="running" color="warning" size="x-small" variant="flat" class="mr-2">Running</v-chip>
                <v-btn size="x-small" variant="text" @click="logLines = []">Clear</v-btn>
              </v-card-title>
              <v-card-text class="pt-0">
                <div ref="logContainer" class="pa-2 rounded" style="max-height: 70vh; overflow-y: auto; background: #0d0d0d; font-family: monospace; font-size: 12px; line-height: 1.4;">
                  <div v-if="logLines.length === 0" class="text-grey">No output yet...</div>
                  <div v-for="(line, i) in logLines" :key="i" :class="logLineClass(line)">{{ line }}</div>
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

    <!-- Help dialog -->
    <v-dialog v-model="showHelpDialog" max-width="680" scrollable>
      <v-card class="help-dialog">
        <v-toolbar color="primary" density="compact">
          <v-icon class="ml-4">mdi-book-open-variant</v-icon>
          <v-toolbar-title class="text-body-1 font-weight-bold ml-2">Lego ACAP Guide</v-toolbar-title>
          <v-spacer />
          <v-btn icon variant="text" @click="showHelpDialog = false"><v-icon>mdi-close</v-icon></v-btn>
        </v-toolbar>

        <v-card-text class="help-content pa-5">

          <!-- Toolbar Actions -->
          <div class="mb-5">
            <div class="d-flex align-center mb-3">
              <v-icon color="info" class="mr-2" size="20">mdi-gesture-tap-button</v-icon>
              <span class="help-heading">Toolbar Actions</span>
            </div>
            <div class="help-item">
              <div class="help-item-header">
                <div :style="{ width: '10px', height: '10px', borderRadius: '50%', background: '#4CAF50', display: 'inline-block' }" />
                <span class="ml-2">Connection Indicator</span>
              </div>
              <div class="help-item-body">Green when the WebSocket is connected, red when disconnected. Reconnects automatically every 3 seconds.</div>
            </div>
            <div class="help-item">
              <div class="help-item-header">
                <v-icon size="14" color="success" class="mr-1">mdi-certificate</v-icon>
                Certificate Chip
              </div>
              <div class="help-item-body">Displays the primary domain. Click to open certificate details with issuer, validity dates, SAN, and download options for <code>.crt</code>, <code>.key</code>, and <code>.issuer.crt</code> files.</div>
            </div>
            <div class="help-item">
              <div class="help-item-header">
                <v-icon size="14" color="warning" class="mr-1">mdi-download</v-icon>
                Download
              </div>
              <div class="help-item-body">Downloads the lego binary from GitHub for the camera's architecture. Progress is shown in real-time. Button turns green with "Binary ready" once available.</div>
            </div>
            <div class="help-item">
              <div class="help-item-header">
                <v-icon size="14" color="success" class="mr-1">mdi-certificate</v-icon>
                Obtain
              </div>
              <div class="help-item-body">Issues a brand-new certificate. Requires the lego binary and a saved configuration. Runs <code>lego run</code> in the background.</div>
            </div>
            <div class="help-item">
              <div class="help-item-header">
                <v-icon size="14" color="secondary" class="mr-1">mdi-refresh</v-icon>
                Renew
              </div>
              <div class="help-item-body">Renews an existing certificate. Only enabled when a certificate is already present. Runs <code>lego renew</code> in the background.</div>
            </div>
            <div class="help-item">
              <div class="help-item-header">
                <v-icon size="14" color="info" class="mr-1">mdi-upload</v-icon>
                Install
              </div>
              <div class="help-item-body">Uploads the certificate to the camera via VAPIX and sets it as the active HTTPS certificate. Each install uses a unique ID; old certificates are cleaned up automatically.</div>
            </div>
            <div class="help-item">
              <div class="help-item-header">
                <v-icon size="14" color="error" class="mr-1">mdi-stop-circle</v-icon>
                Stop
              </div>
              <div class="help-item-body">Cancels a running lego process. Only visible while an obtain or renew is in progress.</div>
            </div>
          </div>

          <!-- Configuration -->
          <div class="mb-5">
            <div class="d-flex align-center mb-3">
              <v-icon color="orange" class="mr-2" size="20">mdi-cog</v-icon>
              <span class="help-heading">Configuration</span>
            </div>

            <div class="help-field">
              <div class="help-field-name">Email <v-chip size="x-small" color="error" variant="flat" class="ml-1">Required</v-chip></div>
              <div class="help-field-desc">ACME account email. Your CA sends certificate expiry notifications here.</div>
            </div>
            <div class="help-field">
              <div class="help-field-name">Domains <v-chip size="x-small" color="error" variant="flat" class="ml-1">Required</v-chip></div>
              <div class="help-field-desc">Comma-separated domain list. First domain = certificate common name. Wildcards supported: <code>*.example.com</code></div>
            </div>
            <div class="help-field">
              <div class="help-field-name">DNS Provider <v-chip size="x-small" color="error" variant="flat" class="ml-1">Required</v-chip></div>
              <div class="help-field-desc">DNS provider for DNS-01 challenge. 100+ providers supported. The list is extracted from the lego binary after download.</div>
            </div>
            <div class="help-field">
              <div class="help-field-name">DNS Resolvers</div>
              <div class="help-field-desc">Resolver for propagation checks. Format: <code>host:port</code>. Default: <code>8.8.8.8:53</code></div>
            </div>
            <div class="help-field">
              <div class="help-field-name">CA Server</div>
              <div class="help-field-desc">ACME directory URL. Defaults to Let's Encrypt production. For testing use:<br/><code>https://acme-staging-v02.api.letsencrypt.org/directory</code></div>
            </div>
            <div class="help-field">
              <div class="help-field-name">Key Type</div>
              <div class="help-field-desc">Private key algorithm. Choose from <code>ec256</code>, <code>ec384</code>, <code>rsa2048</code>, <code>rsa4096</code>. Default: <code>ec256</code></div>
            </div>
          </div>

          <!-- EAB -->
          <div class="mb-5">
            <div class="d-flex align-center mb-3">
              <v-icon color="purple" class="mr-2" size="20">mdi-shield-key</v-icon>
              <span class="help-heading">External Account Binding (EAB)</span>
            </div>
            <p>Some CAs require EAB for account registration. Enable the toggle and enter the credentials from your CA.</p>
            <div class="help-field">
              <div class="help-field-name">EAB Key ID</div>
              <div class="help-field-desc">Key identifier provided by your CA (e.g. ZeroSSL, Google Trust Services, Sectigo).</div>
            </div>
            <div class="help-field">
              <div class="help-field-name">EAB HMAC</div>
              <div class="help-field-desc">Base64 URL-encoded MAC key. Click the <v-icon size="12">mdi-eye</v-icon> icon to reveal the value.</div>
            </div>
          </div>

          <!-- Env Vars -->
          <div class="mb-5">
            <div class="d-flex align-center mb-3">
              <v-icon color="teal" class="mr-2" size="20">mdi-key-variant</v-icon>
              <span class="help-heading">Provider Environment Variables</span>
            </div>
            <p>Each DNS provider needs specific API credentials. These are passed as environment variables to the lego process.</p>
            <div class="help-callout-sm">
              <strong>Examples:</strong>
              <code>CF_DNS_API_TOKEN</code> for Cloudflare,
              <code>AWS_ACCESS_KEY_ID</code> + <code>AWS_SECRET_ACCESS_KEY</code> for Route53
            </div>
            <p>All values are masked by default. Use the <v-icon size="12">mdi-eye</v-icon> icon to toggle visibility. See the <a href="https://go-acme.github.io/lego/dns/" target="_blank">lego DNS provider docs</a> for your provider's requirements.</p>
          </div>

          <!-- Automation -->
          <div class="mb-5">
            <div class="d-flex align-center mb-3">
              <v-icon color="cyan" class="mr-2" size="20">mdi-autorenew</v-icon>
              <span class="help-heading">Automation</span>
            </div>
            <div class="help-field">
              <div class="help-field-name">Auto Mode</div>
              <div class="help-field-desc">When enabled, the app checks every 24 hours whether the certificate needs renewal. If it expires within the configured threshold, it renews and installs automatically. An initial check runs 30 seconds after app startup.</div>
            </div>
            <div class="help-field">
              <div class="help-field-name">Days Before Expiry</div>
              <div class="help-field-desc">Renewal threshold in days. Certificate is renewed when it expires within this window. Default: <code>30</code> days.</div>
            </div>
          </div>

          <!-- Install Process -->
          <div class="mb-5">
            <div class="d-flex align-center mb-3">
              <v-icon color="blue" class="mr-2" size="20">mdi-lan-connect</v-icon>
              <span class="help-heading">Camera Installation Process</span>
            </div>
            <div class="help-steps">
              <div class="help-step">
                <div class="help-step-num" style="background: #1976D2;">1</div>
                <div>Uploads certificate + private key to the camera via VAPIX <code>LoadCertificateWithPrivateKey</code></div>
              </div>
              <div class="help-step">
                <div class="help-step-num" style="background: #1976D2;">2</div>
                <div>Configures HTTPS to use the new certificate via <code>SetWebServerTlsConfiguration</code></div>
              </div>
              <div class="help-step">
                <div class="help-step-num" style="background: #1976D2;">3</div>
                <div>Cleans up old <code>lego-*</code> certificates from the camera</div>
              </div>
            </div>
            <p class="mt-2 text-caption text-grey">VAPIX credentials are retrieved via D-Bus at startup. If unavailable, the Install button and auto-install are disabled.</p>
          </div>

          <!-- Log -->
          <div>
            <div class="d-flex align-center mb-3">
              <v-icon color="grey" class="mr-2" size="20">mdi-console</v-icon>
              <span class="help-heading">Log Output</span>
            </div>
            <p>Streams real-time output from the lego process via WebSocket. The <strong>Last Run</strong> panel stores output from the most recent operation for review.</p>
            <div class="d-flex align-center flex-wrap ga-2 mt-2">
              <span class="text-caption text-grey mr-1">Color coding:</span>
              <v-chip size="x-small" color="error" variant="tonal">ERROR</v-chip>
              <v-chip size="x-small" color="warning" variant="tonal">WARN</v-chip>
              <v-chip size="x-small" color="success" variant="tonal">INFO</v-chip>
              <v-chip size="x-small" color="info" variant="tonal">--- Markers ---</v-chip>
              <v-chip size="x-small" color="purple" variant="tonal">Running: command</v-chip>
            </div>
          </div>

        </v-card-text>
      </v-card>
    </v-dialog>

    <v-snackbar v-model="snackbar" :color="snackbarColor" :timeout="3000">
      {{ snackbarText }}
    </v-snackbar>
  </v-app>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, nextTick } from 'vue'

interface LastRun {
  id: number
  created_at: string
  command: string
  success: boolean
  output: string
}

interface CertInfo {
  has_cert: boolean
  domain: string
  issuer?: string
  not_before?: string
  not_after?: string
  san?: string[]
  serial?: string
}

const WS = {
  DOWNLOAD_PROGRESS: 'download_progress',
  DOWNLOAD_COMPLETE: 'download_complete',
  DOWNLOAD_ERROR: 'download_error',
  LEGO_OUTPUT: 'lego_output',
  LEGO_COMPLETE: 'lego_complete',
  LEGO_ERROR: 'lego_error',
} as const

const MAX_LOG_LINES = 1000

const isDev = import.meta.env.DEV
const baseUrl = isDev ? '/api' : './api'
const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
const wsUrl = isDev
  ? `ws://${window.location.host}/ws`
  : `${wsProtocol}//${window.location.host}${window.location.pathname.replace(/\/[^/]*$/, '')}/../legoacap_ws/ws`

const wsConnected = ref(false)
const legoReady = ref(false)
const downloading = ref(false)
const downloadPercent = ref(0)
const downloadMessage = ref('')
const running = ref(false)
const saving = ref(false)
const envVarVisible = reactive<Record<string, boolean>>({})
const showEabHmac = ref(false)
const showCertDialog = ref(false)
const showHelpDialog = ref(false)
const installing = ref(false)
const showLastRunLog = ref(false)
const lastRun = ref<LastRun | null>(null)
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
  auto_mode: false,
  auto_days: 30,
})

const envVars = reactive<Record<string, string>>({})
const newEnvKey = ref('')
const newEnvValue = ref('')
const certInfo = ref<CertInfo | null>(null)
const dnsProviders = ref<string[]>([])
const keyTypes = ['ec256', 'ec384', 'rsa2048', 'rsa4096']

const rules = {
  required: (v: string) => !!v || 'Required',
  email: (v: string) => /.+@.+\..+/.test(v) || 'Invalid email',
  url: (v: string) => !v || /^https?:\/\/.+/.test(v) || 'Must be a valid URL (https://...)',
  dnsResolvers: (v: string) => !v || /^[\w.:, ]+$/.test(v) || 'Invalid format, e.g. 8.8.8.8:53',
  nonNegativeInt: (v: number) => v >= 0 || 'Must be 0 or greater',
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

async function errorFromResponse(res: Response, fallback: string): Promise<string> {
  try {
    const data = await res.json()
    return data.error || fallback
  } catch {
    return fallback
  }
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
      showMessage(await errorFromResponse(res, 'Failed to save'), 'error')
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
    const res = await fetch(`${baseUrl}/download`, { method: 'POST' })
    if (!res.ok) {
      showMessage(await errorFromResponse(res, 'Failed to start download'), 'error')
      downloading.value = false
    }
  } catch {
    showMessage('Failed to start download', 'error')
    downloading.value = false
  }
}

async function obtainCert() {
  running.value = true
  logLines.value.push('--- Starting certificate obtain ---')
  try {
    const res = await fetch(`${baseUrl}/obtain`, { method: 'POST' })
    if (!res.ok) {
      showMessage(await errorFromResponse(res, 'Failed to start obtain'), 'error')
      running.value = false
    }
  } catch {
    showMessage('Failed to start obtain', 'error')
    running.value = false
  }
}

async function renewCert() {
  running.value = true
  logLines.value.push('--- Starting certificate renewal ---')
  try {
    const res = await fetch(`${baseUrl}/renew`, { method: 'POST' })
    if (!res.ok) {
      showMessage(await errorFromResponse(res, 'Failed to start renewal'), 'error')
      running.value = false
    }
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

async function installCert() {
  installing.value = true
  try {
    const res = await fetch(`${baseUrl}/cert/install`, { method: 'POST' })
    const data = await res.json()
    if (res.ok) {
      showMessage(data.message || 'Certificate installed to camera')
    } else {
      showMessage(data.error || 'Failed to install certificate', 'error')
    }
  } catch {
    showMessage('Failed to install certificate', 'error')
  } finally {
    installing.value = false
  }
}

async function fetchLastRun() {
  try {
    const res = await fetch(`${baseUrl}/runs/last`)
    if (res.ok) {
      lastRun.value = await res.json()
    }
  } catch { /* ignore */ }
}

function downloadCertFile(type: string) {
  window.open(`${baseUrl}/cert/download/${type}`, '_blank')
}

function logLineClass(line: string): string {
  if (line.startsWith('ERROR') || line.includes('[ERROR]')) return 'log-error'
  if (line.includes('[WARN]') || line.includes('[WARNING]')) return 'log-warn'
  if (line.includes('[INFO]')) return 'log-info'
  if (line.startsWith('---')) return 'log-marker'
  if (line.startsWith('Running:')) return 'log-cmd'
  return 'text-grey-lighten-2'
}

function appendLog(line: string) {
  logLines.value.push(line)
  if (logLines.value.length > MAX_LOG_LINES) {
    logLines.value = logLines.value.slice(-MAX_LOG_LINES)
  }
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
        case WS.DOWNLOAD_PROGRESS:
          downloading.value = true
          downloadPercent.value = msg.data.percent || 0
          downloadMessage.value = msg.data.message || ''
          break
        case WS.DOWNLOAD_COMPLETE:
          downloading.value = false
          legoReady.value = true
          downloadPercent.value = 100
          showMessage(msg.data.message || 'Download complete')
          fetchProviders()
          break
        case WS.DOWNLOAD_ERROR:
          downloading.value = false
          showMessage(msg.data.error || 'Download failed', 'error')
          break
        case WS.LEGO_OUTPUT:
          appendLog(msg.data.line || '')
          break
        case WS.LEGO_COMPLETE:
          running.value = false
          appendLog('--- ' + (msg.data.message || 'Done') + ' ---')
          showMessage(msg.data.message || 'Done')
          fetchCertInfo()
          fetchLastRun()
          break
        case WS.LEGO_ERROR:
          running.value = false
          appendLog('ERROR: ' + (msg.data.error || 'Unknown error'))
          showMessage(msg.data.error || 'Lego error', 'error')
          fetchLastRun()
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
  fetchLastRun()
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
.help-content p { font-size: 0.85rem; margin-bottom: 6px; line-height: 1.5; }
.help-content a { color: rgb(var(--v-theme-primary)); }
.help-content code { background: rgba(255,255,255,0.08); padding: 1px 5px; border-radius: 3px; font-size: 0.8rem; }
.help-heading { font-size: 0.95rem; font-weight: 700; letter-spacing: 0.02em; }
.help-callout { background: rgba(76, 175, 80, 0.08); border: 1px solid rgba(76, 175, 80, 0.25); border-radius: 8px; padding: 16px; }
.help-callout-sm { background: rgba(255, 255, 255, 0.04); border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 6px; padding: 10px 14px; margin: 8px 0; font-size: 0.85rem; }
.help-steps { display: flex; flex-direction: column; gap: 10px; }
.help-step { display: flex; align-items: flex-start; gap: 12px; font-size: 0.85rem; line-height: 1.5; }
.help-step-num { min-width: 24px; height: 24px; border-radius: 50%; background: #4CAF50; color: #fff; display: flex; align-items: center; justify-content: center; font-size: 0.75rem; font-weight: 700; flex-shrink: 0; }
.help-item { border-left: 3px solid rgba(255, 255, 255, 0.1); padding: 6px 0 6px 12px; margin-bottom: 8px; }
.help-item-header { font-size: 0.85rem; font-weight: 600; display: flex; align-items: center; margin-bottom: 2px; }
.help-item-body { font-size: 0.82rem; color: rgba(255, 255, 255, 0.65); line-height: 1.5; }
.help-field { background: rgba(255, 255, 255, 0.03); border-radius: 6px; padding: 8px 12px; margin-bottom: 6px; }
.help-field-name { font-size: 0.85rem; font-weight: 600; margin-bottom: 2px; display: flex; align-items: center; }
.help-field-desc { font-size: 0.82rem; color: rgba(255, 255, 255, 0.6); line-height: 1.5; }
.log-error { color: #F44336; }
.log-warn { color: #FF9800; }
.log-info { color: #4CAF50; }
.log-marker { color: #64B5F6; font-weight: 600; }
.log-cmd { color: #CE93D8; }
</style>
