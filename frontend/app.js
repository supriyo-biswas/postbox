import Alpine from 'alpinejs'
import ky from 'ky'
import { format } from 'timeago.js'

Alpine.start()
Alpine.store('g', {
  nameAndEmail (name, email) {
    return name ? `${name} <${email}>` : email
  }
})

async function getUserInfo () {
  const response = await ky.get('/web/api/info')
  const { name: inbox, token, emails_count: count } = await response.json()
  document.title = `Inbox: ${inbox}`
  Alpine.store('inbox', inbox)
  Alpine.store('token', token)
  Alpine.store('page', 1)
  Alpine.store('pages', Math.floor(count / 100) + 1)
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
  const inbox = Alpine.store('inbox')
  const token = Alpine.store('token')
  const page = Alpine.store('page')

  const messages = []
  const response = await ky.get(`/api/v1/inboxes/${inbox}/messages?page=${page}&size=50`, {
    headers: { 'Api-Token': token }
  })

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
    const inbox = Alpine.store('inbox')
    const token = Alpine.store('token')

    const messageUrl = `/api/v1/inboxes/${inbox}/messages/${id}`
    const headers = { 'Api-Token': token }

    const responses = await Promise.allSettled([
      ky.get(`${messageUrl}/body.txt`, { headers }),
      ky.get(`${messageUrl}/attachments`, { headers }),
      ky.patch(messageUrl, { headers, json: { message: { is_read: true } } })
    ])

    const rejection = responses.find(response => response.status === 'rejected')
    if (rejection) {
      throw rejection.reason
    }

    const text = await responses[0].value.text()
    const attachments = await responses[1].value.json()
    dispatch('messageloaded', { body: escapeAndLinkify(text), attachments })
  } catch (e) {
    handleError(e)
  }
})

getUserInfo().then(loadMessages).catch(handleError)
window.addEventListener('loadmessages', loadMessages)
