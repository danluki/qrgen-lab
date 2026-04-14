<script setup>
import { computed, onBeforeUnmount, ref } from 'vue'

const form = ref({ content: '', size: 256 })
const task = ref(null)
const loading = ref(false)
const error = ref('')
const imageVersion = ref(Date.now())
let pollHandle = null

const statusText = computed(() => task.value?.status ?? 'idle')
const imageUrl = computed(() => {
  if (!task.value || task.value.status !== 'completed') {
    return ''
  }
  return `${task.value.image_url}?t=${imageVersion.value}`
})

async function createTask() {
  stopPolling()
  error.value = ''
  task.value = null
  loading.value = true

  try {
    const response = await fetch('/api/tasks', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        content: form.value.content,
        size: Number(form.value.size),
      }),
    })

    const data = await response.json()
    if (!response.ok) {
      throw new Error(data.error || 'Не удалось создать задачу')
    }

    task.value = data.task
    startPolling(task.value.id)
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

async function refreshTask(taskId) {
  const response = await fetch(`/api/tasks/${taskId}`)
  const data = await response.json()
  if (!response.ok) {
    throw new Error(data.error || 'Не удалось получить статус')
  }

  task.value = data.task
  if (task.value.status === 'completed' || task.value.status === 'failed') {
    imageVersion.value = Date.now()
    stopPolling()
  }
}

function startPolling(taskId) {
  refreshTask(taskId).catch((err) => {
    error.value = err.message
    stopPolling()
  })

  pollHandle = window.setInterval(async () => {
    try {
      await refreshTask(taskId)
    } catch (err) {
      error.value = err.message
      stopPolling()
    }
  }, 1500)
}

function stopPolling() {
  if (pollHandle) {
    window.clearInterval(pollHandle)
    pollHandle = null
  }
}

onBeforeUnmount(() => {
  stopPolling()
})
</script>

<template>
  <main class="page-shell">
    <section class="hero-card">
      <p class="eyebrow">Docker Compose Microservices</p>
      <h1>Генератор QR-кодов</h1>
      <p class="lead">
        Vue-интерфейс отправляет задачу в Go API, Redis + Asynq ставят её в очередь,
        а сервис <code>qrgen</code> собирает PNG в фоне.
      </p>

      <form class="generator-form" @submit.prevent="createTask">
        <label>
          Текст или ссылка
          <textarea
            v-model="form.content"
            rows="4"
            maxlength="600"
            placeholder="Например: https://example.com/invite/42"
            required
          />
        </label>

        <label>
          Размер QR-кода
          <input v-model="form.size" type="range" min="128" max="1024" step="32" />
          <span class="range-value">{{ form.size }} px</span>
        </label>

        <button :disabled="loading" type="submit">
          {{ loading ? 'Создание задачи...' : 'Сгенерировать QR-код' }}
        </button>
      </form>
    </section>

    <section class="result-card">
      <div class="result-header">
        <div>
          <p class="eyebrow">Статус задачи</p>
          <h2>{{ statusText }}</h2>
        </div>
        <span v-if="task" class="task-id">{{ task.id }}</span>
      </div>

      <p v-if="error" class="error-box">{{ error }}</p>
      <p v-else-if="!task" class="muted">Создайте задачу, и здесь появится результат.</p>
      <p v-else-if="task.status === 'queued' || task.status === 'processing'" class="muted">
        Задача поставлена в очередь и обрабатывается сервисом <code>qrgen</code>.
      </p>
      <p v-else-if="task.status === 'failed'" class="error-box">
        Обработка завершилась ошибкой: {{ task.error || 'неизвестная ошибка' }}
      </p>

      <div v-if="task?.status === 'completed'" class="preview-block">
        <img :src="imageUrl" alt="Generated QR code" class="qr-image" />
        <a class="download-link" :href="imageUrl" download="qr-code.png">Скачать PNG</a>
      </div>
    </section>
  </main>
</template>
