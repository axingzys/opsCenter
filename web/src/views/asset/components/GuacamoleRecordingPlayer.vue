<template>
  <div ref="playerRootRef" class="guac-player">
    <div ref="displayViewportRef" class="guac-player-stage">
      <div ref="displayHostRef" class="guac-player-display-host"></div>

      <div v-if="loading" class="guac-player-overlay">
        <el-icon class="is-loading guac-player-overlay-icon"><Loading /></el-icon>
        <div class="guac-player-overlay-title">正在加载录屏</div>
        <div class="guac-player-overlay-subtitle">{{ loadingSubtitle }}</div>
        <el-progress
          v-if="loadingPercent > 0"
          class="guac-player-progress"
          :percentage="loadingPercent"
          :stroke-width="6"
        />
      </div>

      <div v-if="errorMessage" class="guac-player-overlay guac-player-overlay-error">
        <div class="guac-player-overlay-title">录屏回放失败</div>
        <div class="guac-player-overlay-subtitle">{{ errorMessage }}</div>
      </div>
    </div>

    <div class="guac-player-toolbar">
      <div class="guac-player-actions">
        <el-button
          :disabled="!canControl"
          circle
          @click="togglePlayback"
        >
          <el-icon><VideoPause v-if="playing" /><VideoPlay v-else /></el-icon>
        </el-button>

        <el-button
          :disabled="!canControl"
          circle
          @click="restartPlayback"
        >
          <el-icon><RefreshRight /></el-icon>
        </el-button>

        <el-button circle @click="toggleFullscreen">
          <el-icon><FullScreen /></el-icon>
        </el-button>
      </div>

      <div class="guac-player-timeline">
        <span class="guac-player-time">{{ formatTime(displayPositionMs) }}</span>
        <el-slider
          v-model="sliderValue"
          class="guac-player-slider"
          :min="0"
          :max="sliderMax"
          :step="1000"
          :show-tooltip="false"
          :disabled="!canSeek"
          @input="handleSliderInput"
          @change="handleSliderChange"
        />
        <span class="guac-player-time">{{ formatTime(durationMs) }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { FullScreen, Loading, RefreshRight, VideoPause, VideoPlay } from '@element-plus/icons-vue'

interface Props {
  src: string
  autoplay?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  autoplay: false
})

const emit = defineEmits<{
  loaded: []
  error: [message: string]
}>()

type GuacamoleTunnel = {
  size?: string | number | null
}

type GuacamoleDisplay = {
  getElement: () => HTMLElement
  getWidth: () => number
  getHeight: () => number
  scale: (scale: number) => void
  onresize?: ((width: number, height: number) => void) | null
}

type GuacamoleRecording = {
  connect: () => void
  disconnect: () => void
  abort: () => void
  play: () => void
  pause: () => void
  seek: (position: number, callback?: () => void) => void
  getDisplay: () => GuacamoleDisplay
  getPosition: () => number
  getDuration: () => number
  isPlaying: () => boolean
  onload?: (() => void) | null
  onerror?: ((message: string) => void) | null
  onprogress?: ((duration: number, parsedSize: number) => void) | null
  onplay?: (() => void) | null
  onpause?: (() => void) | null
  onseek?: ((position: number) => void) | null
}

let guacamoleScriptPromise: Promise<void> | null = null

const GUACAMOLE_COMPAT_PATCH_FLAG = '__opshubGuacamoleCompatPatched'
const CANVAS_DRAWIMAGE_PATCH_FLAG = '__opshubCanvasDrawImageCompatPatched'

const playerRootRef = ref<HTMLDivElement>()
const displayViewportRef = ref<HTMLDivElement>()
const displayHostRef = ref<HTMLDivElement>()

const loading = ref(true)
const errorMessage = ref('')
const playing = ref(false)
const durationMs = ref(0)
const displayPositionMs = ref(0)
const sliderValue = ref(0)
const loadedBytes = ref(0)
const totalBytes = ref<number | null>(null)

let recording: GuacamoleRecording | null = null
let tunnel: GuacamoleTunnel | null = null
let display: GuacamoleDisplay | null = null
let resizeObserver: ResizeObserver | null = null
let fullscreenChangeHandler: (() => void) | null = null
let autoPlayTriggered = false
let isDraggingSlider = false
let positionTimer: number | null = null

const sliderMax = computed(() => Math.max(durationMs.value, 1))
const canControl = computed(() => !loading.value && !errorMessage.value && durationMs.value > 0)
const canSeek = computed(() => canControl.value && durationMs.value > 1000)
const loadingPercent = computed(() => {
  if (!totalBytes.value || totalBytes.value <= 0) {
    return 0
  }
  return Math.min(100, Math.round((loadedBytes.value / totalBytes.value) * 100))
})
const loadingSubtitle = computed(() => {
  if (loadingPercent.value > 0) {
    return `已加载 ${loadingPercent.value}%`
  }
  return '正在初始化回放数据流'
})

const applyCanvasDrawImageCompatPatch = () => {
  const runtimeWindow = window as any
  if (runtimeWindow[CANVAS_DRAWIMAGE_PATCH_FLAG]) {
    return
  }

  const contextPrototype = window.CanvasRenderingContext2D?.prototype as CanvasRenderingContext2D | undefined
  const originalDrawImage = contextPrototype?.drawImage
  if (!originalDrawImage) {
    return
  }

  contextPrototype.drawImage = function patchedDrawImage(...args: Parameters<CanvasRenderingContext2D['drawImage']>) {
    const source = args[0] as any
    const isCanvasSource = typeof HTMLCanvasElement !== 'undefined' && source instanceof HTMLCanvasElement
    const sourceWidth = Number(source?.width || 0)
    const sourceHeight = Number(source?.height || 0)

    // Guacamole may export hidden 0x0 layers as canvases while generating keyframes.
    if (isCanvasSource && (!sourceWidth || !sourceHeight)) {
      return
    }

    return originalDrawImage.apply(this, args)
  }

  runtimeWindow[CANVAS_DRAWIMAGE_PATCH_FLAG] = true
}

const applyGuacamoleCompatPatches = () => {
  const runtimeWindow = window as any
  if (runtimeWindow[GUACAMOLE_COMPAT_PATCH_FLAG]) {
    return
  }

  const Guacamole = runtimeWindow.Guacamole
  if (!Guacamole?.Layer) {
    return
  }

  const OriginalLayer = Guacamole.Layer
  const PatchedLayer = function patchedLayer(this: any, ...args: any[]) {
    OriginalLayer.apply(this, args)

    const getCanvas = typeof this.getCanvas === 'function' ? this.getCanvas.bind(this) : null
    this.toCanvas = function safeToCanvas() {
      const exportCanvas = document.createElement('canvas')
      const layerWidth = Math.max(Number(this?.width || 0), 0)
      const layerHeight = Math.max(Number(this?.height || 0), 0)

      exportCanvas.width = layerWidth
      exportCanvas.height = layerHeight

      const sourceCanvas = getCanvas?.()
      if (!layerWidth || !layerHeight || !sourceCanvas?.width || !sourceCanvas?.height) {
        return exportCanvas
      }

      const exportContext = exportCanvas.getContext('2d')
      if (!exportContext) {
        return exportCanvas
      }

      exportContext.drawImage(sourceCanvas, 0, 0)
      return exportCanvas
    }
  }

  PatchedLayer.prototype = OriginalLayer.prototype
  PatchedLayer.prototype.constructor = PatchedLayer
  Object.setPrototypeOf(PatchedLayer, OriginalLayer)

  Guacamole.Layer = PatchedLayer
  runtimeWindow[GUACAMOLE_COMPAT_PATCH_FLAG] = true
}

const loadGuacamoleLibrary = async () => {
  if ((window as any).Guacamole) {
    applyCanvasDrawImageCompatPatch()
    applyGuacamoleCompatPatches()
    return
  }

  if (!guacamoleScriptPromise) {
    guacamoleScriptPromise = new Promise<void>((resolve, reject) => {
      const script = document.createElement('script')
      script.src = '/guacamole/guacamole-common-js/all.min.js'
      script.onload = () => resolve()
      script.onerror = () => reject(new Error('加载 Guacamole 播放器失败'))
      document.head.appendChild(script)
    })
  }

  await guacamoleScriptPromise
  applyCanvasDrawImageCompatPatch()
  applyGuacamoleCompatPatches()
}

const buildTunnelHeaders = () => {
  const token = localStorage.getItem('token')
  if (!token) {
    return {}
  }
  return {
    Authorization: `Bearer ${token}`
  }
}

const syncPosition = (position: number) => {
  displayPositionMs.value = Math.max(0, position || 0)
  if (!isDraggingSlider) {
    sliderValue.value = displayPositionMs.value
  }
}

const fitDisplay = () => {
  if (!display || !displayViewportRef.value) {
    return
  }

  const width = display.getWidth()
  const height = display.getHeight()
  if (!width || !height) {
    return
  }

  const availableWidth = Math.max(displayViewportRef.value.clientWidth - 24, 1)
  const availableHeight = Math.max(displayViewportRef.value.clientHeight - 24, 1)
  const scale = Math.max(Math.min(availableWidth / width, availableHeight / height), 0.1)
  display.scale(scale)
}

const hasRenderableDisplay = () => {
  if (!display) {
    return false
  }

  return display.getWidth() > 0 && display.getHeight() > 0
}

const stopPositionTimer = () => {
  if (positionTimer !== null) {
    window.clearInterval(positionTimer)
    positionTimer = null
  }
}

const startPositionTimer = () => {
  stopPositionTimer()
  positionTimer = window.setInterval(() => {
    if (!recording || !playing.value || isDraggingSlider) {
      return
    }
    syncPosition(recording.getPosition())
  }, 200)
}

const cleanupPlayer = () => {
  stopPositionTimer()

  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }

  if (fullscreenChangeHandler) {
    document.removeEventListener('fullscreenchange', fullscreenChangeHandler)
    fullscreenChangeHandler = null
  }

  if (recording) {
    try {
      recording.pause()
      recording.abort()
      recording.disconnect()
    } catch {
      // ignore player cleanup errors
    }
    recording = null
  }

  tunnel = null
  display = null

  if (displayHostRef.value) {
    displayHostRef.value.innerHTML = ''
  }
}

const attemptAutoplay = () => {
  if (!props.autoplay || autoPlayTriggered || !recording || durationMs.value <= 0 || !hasRenderableDisplay()) {
    return
  }

  autoPlayTriggered = true
  recording.play()
}

const initializePlayer = async () => {
  cleanupPlayer()

  loading.value = true
  errorMessage.value = ''
  playing.value = false
  durationMs.value = 0
  displayPositionMs.value = 0
  sliderValue.value = 0
  loadedBytes.value = 0
  totalBytes.value = null
  autoPlayTriggered = false
  isDraggingSlider = false

  if (!props.src || !displayHostRef.value) {
    loading.value = false
    return
  }

  try {
    await loadGuacamoleLibrary()

    const Guacamole = (window as any).Guacamole
    if (!Guacamole?.StaticHTTPTunnel || !Guacamole?.SessionRecording) {
      throw new Error('Guacamole 播放器未正确加载')
    }

    tunnel = new Guacamole.StaticHTTPTunnel(props.src, false, buildTunnelHeaders())
    recording = new Guacamole.SessionRecording(tunnel, 200)
    display = recording.getDisplay()
    displayHostRef.value.appendChild(display.getElement())

    display.onresize = () => {
      fitDisplay()
      attemptAutoplay()
    }

    resizeObserver = new ResizeObserver(() => {
      fitDisplay()
    })
    resizeObserver.observe(displayViewportRef.value!)

    fullscreenChangeHandler = () => {
      fitDisplay()
    }
    document.addEventListener('fullscreenchange', fullscreenChangeHandler)

    recording.onprogress = (duration, parsedSize) => {
      durationMs.value = Math.max(durationMs.value, duration || 0)
      loadedBytes.value = parsedSize || 0

      const size = tunnel?.size
      if (size !== null && size !== undefined && size !== '') {
        const parsed = Number(size)
        if (!Number.isNaN(parsed) && parsed > 0) {
          totalBytes.value = parsed
        }
      }

      if (durationMs.value > 0) {
        loading.value = false
      }

      fitDisplay()
      attemptAutoplay()
    }

    recording.onload = () => {
      loading.value = false
      durationMs.value = Math.max(durationMs.value, recording?.getDuration() || 0)
      syncPosition(recording?.getPosition() || 0)
      fitDisplay()
      attemptAutoplay()
      emit('loaded')
    }

    recording.onerror = (message: string) => {
      errorMessage.value = message || '录屏数据无法加载'
      loading.value = false
      playing.value = false
      stopPositionTimer()
      emit('error', errorMessage.value)
    }

    recording.onplay = () => {
      playing.value = true
      loading.value = false
      startPositionTimer()
    }

    recording.onpause = () => {
      playing.value = false
      syncPosition(recording?.getPosition() || displayPositionMs.value)
      stopPositionTimer()
    }

    recording.onseek = (position: number) => {
      syncPosition(position || 0)
      durationMs.value = Math.max(durationMs.value, recording?.getDuration() || 0)
    }

    recording.connect()

    await nextTick()
    fitDisplay()
    attemptAutoplay()
  } catch (error: any) {
    errorMessage.value = error?.message || '录屏播放器初始化失败'
    loading.value = false
    emit('error', errorMessage.value)
  }
}

const togglePlayback = () => {
  if (!recording || !canControl.value) {
    return
  }

  if (recording.isPlaying()) {
    recording.pause()
    return
  }

  recording.play()
}

const restartPlayback = () => {
  if (!recording || !canControl.value) {
    return
  }

  syncPosition(0)
  recording.seek(0, () => {
    recording?.play()
  })
}

const handleSliderInput = (value: number | number[]) => {
  if (Array.isArray(value)) {
    return
  }
  isDraggingSlider = true
  sliderValue.value = value
  displayPositionMs.value = value
}

const handleSliderChange = (value: number | number[]) => {
  if (Array.isArray(value)) {
    return
  }

  isDraggingSlider = false
  syncPosition(value)
  recording?.seek(value)
}

const toggleFullscreen = async () => {
  const element = playerRootRef.value
  if (!element) {
    return
  }

  if (document.fullscreenElement) {
    await document.exitFullscreen()
    return
  }

  await element.requestFullscreen()
}

const formatTime = (milliseconds: number) => {
  const totalSeconds = Math.max(0, Math.floor(milliseconds / 1000))
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60

  if (hours > 0) {
    return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
  }

  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
}

watch(() => props.src, () => {
  initializePlayer()
})

onMounted(() => {
  initializePlayer()
})

onBeforeUnmount(() => {
  cleanupPlayer()
})
</script>

<style scoped>
.guac-player {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 560px;
  background: #0b1220;
  border-radius: 12px;
  overflow: hidden;
}

.guac-player-stage {
  position: relative;
  flex: 1;
  min-height: 440px;
  padding: 12px;
  background:
    radial-gradient(circle at top, rgba(59, 130, 246, 0.18), transparent 35%),
    linear-gradient(180deg, #111827 0%, #020617 100%);
  overflow: hidden;
}

.guac-player-display-host {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.guac-player-display-host :deep(canvas) {
  image-rendering: auto;
}

.guac-player-overlay {
  position: absolute;
  inset: 0;
  background: rgba(2, 6, 23, 0.82);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: #e5e7eb;
  z-index: 2;
}

.guac-player-overlay-error {
  background: rgba(69, 10, 10, 0.82);
}

.guac-player-overlay-icon {
  font-size: 28px;
  color: #fbbf24;
}

.guac-player-overlay-title {
  font-size: 18px;
  font-weight: 600;
}

.guac-player-overlay-subtitle {
  font-size: 13px;
  color: #cbd5e1;
}

.guac-player-progress {
  width: min(320px, 80%);
}

.guac-player-toolbar {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 14px 18px;
  border-top: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(15, 23, 42, 0.96);
}

.guac-player-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.guac-player-actions :deep(.el-button) {
  background: rgba(30, 41, 59, 0.9);
  border-color: rgba(148, 163, 184, 0.25);
  color: #f8fafc;
}

.guac-player-actions :deep(.el-button:hover) {
  background: rgba(59, 130, 246, 0.24);
  border-color: rgba(96, 165, 250, 0.5);
  color: #dbeafe;
}

.guac-player-actions :deep(.el-button.is-disabled) {
  background: rgba(30, 41, 59, 0.5);
  color: rgba(226, 232, 240, 0.45);
}

.guac-player-timeline {
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 1;
  min-width: 0;
}

.guac-player-slider {
  flex: 1;
  min-width: 0;
}

.guac-player-slider :deep(.el-slider__runway) {
  background: rgba(148, 163, 184, 0.2);
}

.guac-player-slider :deep(.el-slider__bar) {
  background: linear-gradient(90deg, #f59e0b 0%, #3b82f6 100%);
}

.guac-player-slider :deep(.el-slider__button) {
  border-color: #f8fafc;
}

.guac-player-time {
  width: 52px;
  font-family: 'Consolas', 'Monaco', monospace;
  font-size: 12px;
  color: #cbd5e1;
  text-align: center;
  flex-shrink: 0;
}

@media (max-width: 768px) {
  .guac-player {
    min-height: 420px;
  }

  .guac-player-stage {
    min-height: 320px;
  }

  .guac-player-toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .guac-player-actions {
    justify-content: center;
  }

  .guac-player-time {
    width: 44px;
    font-size: 11px;
  }
}
</style>
