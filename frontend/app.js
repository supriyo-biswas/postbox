import '@tabler/icons-webfont/dist/tabler-icons.min.css'
import Alpine from 'alpinejs'
import ky from 'ky'
import { format } from 'timeago.js'

const api = ky.create({ prefixUrl: '/web/api', credentials: 'same-origin' })
const auth = ky.create({ prefixUrl: '/web', credentials: 'same-origin', retry: 0 })
const pageSize = 50

function escape (text) {
  return text.replace(/[<>&"']/g, match => ({ '<': '&lt;', '>': '&gt;', '&': '&amp;', '"': '&quot;', "'": '&#39;' })[match])
}

function escapeAndLinkify (text) {
  return text.replace(/[<>&"']|\bhttps?:\/\/[^>)\s]+/g, match => match.startsWith('http')
    ? `<a href="${escape(match)}" target="_blank" rel="noopener noreferrer">${escape(match)}</a>`
    : escape(match))
}

Alpine.data('mailbox', () => ({
  authenticated: false,
  authChecking: true,
  authBusy: false,
  authError: '',
  username: '',
  password: '',
  client: null,
  requests: null,
  inbox: '',
  count: 0,
  unread: 0,
  messages: [],
  currentMessage: null,
  search: '',
  page: 1,
  hasNext: false,
  loading: true,
  messageLoading: false,
  deletingMessage: false,
  mobileReader: false,
  bodyMode: 'text',
  error: '',
  messageError: '',
  listRequest: 0,
  bodyRequest: 0,
  readUpdates: [],

  async init () {
    this.startRequests()
    await this.loadMessages()
    this.authChecking = false
  },

  startRequests () {
    this.requests?.abort()
    this.requests = new AbortController()
    this.client = api.extend({
      signal: this.requests.signal,
      hooks: {
        afterResponse: [(_request, _options, response) => {
          if (response.status === 401) this.showLogin()
        }]
      }
    })
  },

  showLogin (expired = this.authenticated) {
    this.requests?.abort()
    this.listRequest++
    this.bodyRequest++
    this.authenticated = false
    this.authChecking = false
    this.inbox = ''
    this.messages = []
    this.currentMessage = null
    this.count = 0
    this.unread = 0
    this.search = ''
    this.page = 1
    this.hasNext = false
    this.loading = false
    this.messageLoading = false
    this.messageError = ''
    this.error = ''
    this.readUpdates = []
    this.mobileReader = false
    this.password = ''
    this.authError = expired ? 'Your session expired. Please log in again.' : ''
    document.title = 'Log in · Postbox'
  },

  async login () {
    if (this.authBusy) return
    this.authBusy = true
    this.authError = ''
    try {
      await auth.post('auth', { json: { username: this.username, password: this.password } })
      this.startRequests()
      await this.loadMessages(true)
    } catch (error) {
      this.authError = error.response?.status === 401
        ? 'Invalid username or password.'
        : 'Unable to log in. Please try again.'
    } finally {
      this.password = ''
      this.authBusy = false
    }
  },

  async logout () {
    if (this.authBusy) return
    this.authBusy = true
    this.error = ''
    try {
      await auth.delete('auth')
      this.showLogin(false)
    } catch (error) {
      if (error.response?.status === 401) this.showLogin()
      else this.error = 'Unable to log out. Please try again.'
    } finally {
      this.authBusy = false
    }
  },

  async loadMessages (reset = false) {
    if (reset) this.page = 1
    const request = ++this.listRequest
    this.loading = true
    this.error = ''
    const search = this.search.trim()
    try {
      const [info, messages] = await Promise.all([
        this.client.get('').json(),
        this.client.get('messages', { searchParams: { page: this.page, size: pageSize, search } }).json()
      ])
      if (request !== this.listRequest) return
      this.inbox = info.username
      this.username = info.username
      this.authenticated = true
      this.authError = ''
      this.count = info.emails_count
      this.unread = info.emails_unread_count
      document.title = `${this.inbox} · Postbox`
      this.messages = messages
      this.hasNext = search ? messages.length === pageSize : this.page * pageSize < this.count
      if (this.currentMessage && !messages.some(message => message.id === this.currentMessage.id)) {
        this.currentMessage = null
        this.mobileReader = false
        this.bodyRequest++
        this.messageLoading = false
      }
      if (!this.currentMessage && messages.length && window.matchMedia('(min-width: 721px)').matches) {
        this.openMessage(messages[0], false)
      }
    } catch (error) {
      if (request === this.listRequest) {
        if (this.authenticated) this.error = error.message || String(error)
        else this.authError = 'Unable to load your inbox. Please try again.'
      }
    } finally {
      if (request === this.listRequest) this.loading = false
    }
  },

  async openMessage (message, showOnMobile = true) {
    this.mobileReader = showOnMobile
    if (this.currentMessage?.id === message.id && !this.messageError) return
    const request = ++this.bodyRequest
    this.currentMessage = { ...message, textBody: '', htmlBody: '', attachments: [] }
    this.messageLoading = true
    this.messageError = ''
    this.bodyMode = message.html_body_size > 0 ? 'html' : 'text'
    try {
      const url = `messages/${message.id}`
      await this.client.get(url)
      const [textResponse, htmlResponse, attachments] = await Promise.all([
        this.client.get(`${url}/body.txt`, { throwHttpErrors: false }),
        this.client.get(`${url}/body.html`, { throwHttpErrors: false }),
        this.client.get(`${url}/attachments`).json(),
        this.client.patch(url, { json: { message: { is_read: true } } })
      ])
      for (const response of [textResponse, htmlResponse]) {
        if (!response.ok && response.status !== 404) throw new Error(`Unable to load message (${response.status})`)
      }
      const [text, html] = await Promise.all([
        textResponse.ok ? textResponse.text() : 'Text body unavailable.',
        htmlResponse.ok ? htmlResponse.text() : '<p>HTML body unavailable.</p>'
      ])
      if (request !== this.bodyRequest) return
      if (!htmlResponse.ok && this.bodyMode === 'html') this.bodyMode = 'text'
      this.currentMessage.textBody = escapeAndLinkify(text)
      this.currentMessage.htmlBody = `<!doctype html><html><head><meta name="viewport" content="width=device-width, initial-scale=1"><style>
        body { margin: 0; padding: 20px 24px; font: 14px/1.6 -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; color: #526078; overflow-wrap: anywhere; }
        a { color: #4777df; }
        @media (max-width: 600px) { body { padding: 18px 16px; } }
      </style></head><body>${html}</body></html>`
      this.currentMessage.attachments = attachments || []
      const row = this.messages.find(item => item.id === message.id)
      if (row && !row.is_read) {
        row.is_read = true
        this.unread = Math.max(0, this.unread - 1)
      }
    } catch (error) {
      if (request === this.bodyRequest) this.messageError = error.message || String(error)
    } finally {
      if (request === this.bodyRequest) this.messageLoading = false
    }
  },

  async toggleRead (message) {
    const isRead = !message.is_read
    this.readUpdates.push(message.id)
    this.error = ''
    try {
      await this.client.patch(`messages/${message.id}`, { json: { message: { is_read: isRead } } })
      const row = this.messages.find(item => item.id === message.id)
      if (row && row.is_read !== isRead) {
        row.is_read = isRead
        this.unread = Math.max(0, this.unread + (isRead ? -1 : 1))
      }
      if (this.currentMessage?.id === message.id) this.currentMessage.is_read = isRead
    } catch (error) {
      this.error = error.message || String(error)
    } finally {
      this.readUpdates = this.readUpdates.filter(id => id !== message.id)
    }
  },

  async deleteMessage () {
    const message = this.currentMessage
    if (!message || !window.confirm('Delete this message? This cannot be undone.')) return
    this.deletingMessage = true
    try {
      await this.client.delete(`messages/${message.id}`)
      if (this.currentMessage?.id === message.id) {
        this.currentMessage = null
        this.mobileReader = false
        this.bodyRequest++
        this.messageLoading = false
      }
      if (this.page > 1 && this.messages.length === 1) this.page--
      await this.loadMessages()
    } catch (error) {
      this.error = `Unable to delete message: ${error.message || String(error)}`
    } finally {
      this.deletingMessage = false
    }
  },

  async clearInbox () {
    if (!window.confirm('Delete all messages in this inbox? This cannot be undone.')) return
    this.loading = true
    this.error = ''
    try {
      await this.client.patch('clean')
      this.search = ''
      this.currentMessage = null
      this.mobileReader = false
      this.bodyRequest++
      await this.loadMessages(true)
    } catch (error) {
      this.error = error.message || String(error)
    } finally {
      this.loading = false
    }
  },

  changePage (direction) {
    this.page += direction
    this.loadMessages()
  },

  initials (message) {
    return (message.from_name || message.from_email || '?').split(/[\s@._-]+/).filter(Boolean).slice(0, 2).map(part => part[0]).join('').toUpperCase()
  },

  senderColor (message) {
    return ['lavender', 'blue', 'rose', 'green', 'amber'][message.id % 5]
  },

  relativeDate (value) {
    return format(new Date(value))
  },

  fullDate (value) {
    return new Date(value).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
  },

  recipients (addresses) {
    return (addresses || []).map(({ name, address }) => name ? `${name} <${address}>` : address).join(', ')
  }
}))

Alpine.start()
