import Alpine from 'alpinejs'
import ky from 'ky'
import { format } from 'timeago.js'

Alpine.start()

async function getUserInfo () {
  const response = await ky.get('/web/api/info')
  const { name: inbox, token } = await response.json()
  document.title = `Inbox: ${inbox}`
  Alpine.store('inbox', inbox)
  Alpine.store('token', token)
}

function dispatch (name, detail) {
  window.dispatchEvent(new CustomEvent(name, { detail }))
}

function handleError (e) {
  console.log(e)
  dispatch('erroroccurred', { error: e instanceof Error ? e.message : e.toString() })
}

async function escapeAndLinkify (text) {
  return text.replace(/[<>&"']|https?:\/\/[^)\]> ]+/g, match => {
    switch (match) {
      case '<': return '&lt;'
      case '>': return '&gt;'
      case '&': return '&amp;'
      case '"': return '&quot;'
      case '\'': return '&#39;'
      default: return `<a href="${match}" target="_blank">${match}</a>`
    }
  })
}

async function loadMessages () {
  const inbox = Alpine.store('inbox')
  const token = Alpine.store('token')

  const messages = []
  let page = 1

  while (true) {
    const response = await ky.get(`/api/v1/inboxes/${inbox}/messages?page=${page}`, {
      headers: { 'Api-Token': token }
    })

    const currentMessages = await response.json()
    if (currentMessages.length === 0 || page > 30) {
      break
    }

    page++

    for (const message of currentMessages) {
      message.created_at = format(new Date(message.created_at))
      message.updated_at = format(new Date(message.updated_at))
      message.sent_at = format(new Date(message.sent_at))
      messages.push(message)
    }
  }

  dispatch('messagesloaded', { messages })
}

document.addEventListener('openmessage', async event => {
  try {
    const id = event.detail
    const inbox = Alpine.store('inbox')
    const token = Alpine.store('token')

    const messageUrl = `/api/v1/inboxes/${inbox}/messages/${id}`
    const responses = await Promise.allSettled([
      ky.get(`${messageUrl}/body.txt`, { headers: { 'Api-Token': token } }),
      ky.patch(messageUrl, { headers: { 'Api-Token': token }, json: { message: { is_read: true } } })
    ])

    if (responses[0].status === 'rejected') {
      throw responses[0].reason
    }

    const text = await responses[0].value.text()
    dispatch('messageloaded', escapeAndLinkify(text))
  } catch (e) {
    handleError(e)
  }
})

getUserInfo().then(loadMessages).catch(handleError)
