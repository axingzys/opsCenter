<template>
  <div class="asciinema-player-container">
    <div class="asciinema-player-wrapper" ref="playerRef"></div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { create } from 'asciinema-player'
import 'asciinema-player/dist/bundle/asciinema-player.css'

interface Props {
  src: string
  cols?: number
  rows?: number
  autoplay?: boolean
  preload?: boolean
  startTime?: number
  speed?: number
  loop?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  cols: 80,
  rows: 24,
  autoplay: false,
  preload: true,
  startTime: 0,
  speed: 1,
  loop: false
})

const emit = defineEmits(['ready', 'play', 'pause', 'finish', 'error'])

const playerRef = ref<HTMLDivElement>()
let player: any = null

const destroyPlayer = () => {
  if (player?.dispose) {
    player.dispose()
  }
  if (playerRef.value) {
    playerRef.value.innerHTML = ''
  }
  player = null
}

const createPlayer = async () => {
  if (!playerRef.value || !props.src) {
    return
  }

  try {
    await nextTick()
    destroyPlayer()

    player = create(props.src, playerRef.value, {
      autoplay: props.autoplay,
      preload: props.preload ? 'auto' : 'none',
      startTime: props.startTime,
      speed: props.speed,
      loop: props.loop,
      theme: 'tango',
      poster: 'npt:0:01',
      controls: true,
    })

    if (player?.addEventListener) {
      player.addEventListener('ready', () => emit('ready'))
      player.addEventListener('play', () => emit('play'))
      player.addEventListener('pause', () => emit('pause'))
      player.addEventListener('ended', () => emit('finish'))
    }
  } catch (error: any) {
    destroyPlayer()
    emit('error', error?.message || '终端播放器初始化失败')
  }
}

watch(() => props.src, () => {
  createPlayer()
})

onMounted(() => {
  createPlayer()
})

onBeforeUnmount(() => {
  destroyPlayer()
})

defineExpose({
  play: () => player?.play(),
  pause: () => player?.pause(),
  seek: (time: number) => player?.seek(time),
  getDuration: () => player?.getDuration?.(),
  getCurrentTime: () => player?.getCurrentTime?.(),
})
</script>

<style scoped>
.asciinema-player-container {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #000;
  min-height: 500px;
}

.asciinema-player-wrapper {
  width: 100%;
  height: 100%;
  overflow: auto;
}

.asciinema-player-wrapper :deep(.asciinema-player) {
  background-color: #000 !important;
}

.asciinema-player-wrapper :deep(.asciinema-player .ap-terminal) {
  background-color: #000 !important;
}

.asciinema-player-wrapper :deep(.asciinema-player .ap-control-bar) {
  background: rgba(0, 0, 0, 0.9) !important;
  opacity: 1 !important;
  height: auto !important;
  min-height: 48px !important;
}

.asciinema-player-wrapper :deep(.asciinema-player .ap-progress-container) {
  background-color: rgba(212, 175, 55, 0.2) !important;
  height: 6px !important;
}

.asciinema-player-wrapper :deep(.asciinema-player .ap-progress-bar) {
  background-color: #d4af37 !important;
}

.asciinema-player-wrapper :deep(.asciinema-player .ap-controls) {
  color: #d4af37 !important;
  display: flex !important;
  opacity: 1 !important;
}

.asciinema-player-wrapper :deep(.asciinema-player .ap-control-bar) {
  display: block !important;
}

.asciinema-player-wrapper :deep(.asciinema-player .ap-icon-button) {
  display: inline-flex !important;
  color: #d4af37 !important;
}

.asciinema-player-wrapper :deep(.asciinema-player .ap-icon-button:hover) {
  color: #bfa13f !important;
}
</style>
