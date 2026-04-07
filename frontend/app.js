import '@fortawesome/fontawesome-free/css/all.css'
import Alpine from 'alpinejs'
import ky from 'ky'
import { format } from 'timeago.js'

const pageSize = 50

Alpine.start()
Alpine.store('g', {
  nameAndEmail (name, email) {
    return name ? `${name} <${email}>` : email
  }
})

const api = ky.create({ prefixUrl: '/web/api' })

async function getUserInfo () {
  const response = await api.get('info')
  const { username: inbox, emails_count: count } = await response.json()
  document.title = `Inbox: ${inbox}`
  Alpine.store('page', 1)
  Alpine.store('pages', Math.floor(count / pageSize) + 1)
}

function dispatch (name, detail) {
  window.dispatchEvent(new CustomEvent(name, { detail }))
}

function handleError (e) {
  console.log(e)
  dispatch('erroroccurred', { error: e instanceof Error ? e.message : e.toString() })
}

function escape (text) {
  return text.replace(/[<>&"']/g, match => {
    switch (match) {
      case '<': return '&lt;'
      case '>': return '&gt;'
      case '&': return '&amp;'
      case '"': return '&quot;'
      case '\'': return '&#39;'
    }
  })
}

function escapeAndLinkify (text) {
  return text.replace(/[<>&"']|\bhttps?:\/\/[^>)\s]+/g, match => {
    if (match.startsWith('http')) {
      return `<a href="${escape(match)}" class="text-blue-500 hover:underline">${escape(match)}</a>`
    }

    return escape(match)
  })
}

async function loadMessages () {
  const page = Alpine.store('page')
  const messages = []
  const response = await api.get(`messages?page=${page}&size=${pageSize}`)

  const currentMessages = await response.json()
  for (const message of currentMessages) {
    message.created_at = format(new Date(message.created_at))
    message.updated_at = format(new Date(message.updated_at))
    message.sent_at = format(new Date(message.sent_at))
    messages.push(message)
  }

  dispatch('messagesloaded', { messages })
}

document.addEventListener('openmessage', async event => {
  try {
    const id = event.detail
    const messageUrl = `messages/${id}`

    // Validate message existence; this should fail fast for deleted/missing messages.
    await api.get(messageUrl)

    const responses = await Promise.allSettled([
      api.get(`${messageUrl}/body.txt`, { throwHttpErrors: false }),
      api.get(`${messageUrl}/body.html`, { throwHttpErrors: false }),
      api.get(`${messageUrl}/attachments`),
      api.patch(messageUrl, { json: { message: { is_read: true } } })
    ])

    const rejection = responses.find(response => response.status === 'rejected')
    if (rejection) {
      throw rejection.reason
    }

    let text = 'Text body unavailable'
    if (responses[0].status === 'fulfilled') {
      if (responses[0].value.ok) {
        text = await responses[0].value.text()
      } else if (responses[0].value.status !== 404) {
        throw new Error(`failed to load text body (${responses[0].value.status})`)
      }
    }

    let html = 'HTML body unavailable'
    if (responses[1].status === 'fulfilled') {
      if (responses[1].value.ok) {
        html = await responses[1].value.text()
      } else if (responses[1].value.status !== 404) {
        throw new Error(`failed to load html body (${responses[1].value.status})`)
      }
    }

    const attachments = await responses[2].value.json()
    dispatch('messageloaded', {
      textBody: escapeAndLinkify(text),
      htmlBody: html,
      attachments
    })
  } catch (e) {
    handleError(e)
  }
})

document.addEventListener('deletemessages', async () => {
  try {
    await api.patch('clean')
    await getUserInfo().then(loadMessages)
  } catch (e) {
    handleError(e)
  }
})

getUserInfo().then(loadMessages).catch(handleError)
document.addEventListener('loadmessages', loadMessages)
