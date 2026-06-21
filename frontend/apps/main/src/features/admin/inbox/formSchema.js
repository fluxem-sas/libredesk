import * as z from 'zod'
import { isGoDuration, validateEmail, isValidTemplate } from '@shared-ui/utils/string'
import { AUTH_TYPE_PASSWORD, AUTH_TYPE_OAUTH2, PROVIDER_RESEND } from '@main/constants/auth.js'

const FROM_NAME_TEMPLATE_VARS = ['.Agent.FirstName', '.Agent.LastName', '.Agent.FullName', '.Inbox.Name']

const requireField = (ctx, path, message) => {
  ctx.addIssue({
    code: z.ZodIssueCode.custom,
    path,
    message
  })
}

export const createFormSchema = (t) => z.object({
  name: z.string().min(1, t('globals.messages.required')),
  application_id: z.number().nullable().optional(),
  from: z.string().min(1, t('globals.messages.required')),
  from_name_template: z
    .string()
    .optional()
    .default('')
    .refine((val) => isValidTemplate(val, FROM_NAME_TEMPLATE_VARS), {
      message: t('admin.inbox.fromNameTemplate.invalidTemplate')
    }),
  reply_to: z
    .string()
    .optional()
    .refine((v) => !v || validateEmail(v), {
      message: t('validation.invalidEmail')
    }),
  enabled: z.boolean().optional(),
  csat_enabled: z.boolean().optional(),
  prompt_tags_on_reply: z.boolean().optional(),
  enable_plus_addressing: z.boolean().optional(),
  auth_type: z.enum([AUTH_TYPE_PASSWORD, AUTH_TYPE_OAUTH2]),
  provider: z.string().optional().default('manual'),
  oauth: z.object({
    access_token: z.string().optional(),
    client_id: z.string().optional(),
    client_secret: z.string().optional(),
    expires_at: z.string().optional(),
    provider: z.string().optional(),
    refresh_token: z.string().optional(),
    tenant_id: z.string().optional()
  }).optional(),
  resend: z.object({
    api_key: z.string().optional(),
    webhook_secret: z.string().optional()
  }).optional(),
  imap: z.object({
    host: z.string().optional().default(''),
    port: z.number().optional().default(993),
    mailbox: z.string().optional().default('INBOX'),
    username: z.string().optional().default(''),
    password: z.string().optional().default(''),
    tls_type: z.enum(['none', 'starttls', 'tls']).default('none'),
    tls_skip_verify: z.boolean().optional(),
    scan_inbox_since: z.string().optional().default('48h'),
    read_interval: z.string().optional().default('5m')
  }),
  smtp: z.object({
    host: z.string().optional().default(''),
    port: z.number().optional().default(587),
    username: z.string().optional().default(''),
    password: z.string().optional().default(''),
    max_conns: z.number().optional().default(10),
    max_msg_retries: z.number().optional().default(3),
    idle_timeout: z.string().optional().default('25s'),
    pool_wait_timeout: z.string().optional().default('120s'),
    tls_type: z.enum(['none', 'starttls', 'tls']).default('none'),
    tls_skip_verify: z.boolean().optional(),
    hello_hostname: z.string().optional(),
    auth_protocol: z.enum(['login', 'cram', 'plain', 'none']).default('login')
  })
}).superRefine((values, ctx) => {
  if (values.provider === PROVIDER_RESEND) {
    if (!values.resend?.api_key) {
      requireField(ctx, ['resend', 'api_key'], t('globals.messages.required'))
    }
    if (!values.resend?.webhook_secret) {
      requireField(ctx, ['resend', 'webhook_secret'], t('globals.messages.required'))
    }
    return
  }

  if (!values.imap?.host) requireField(ctx, ['imap', 'host'], t('globals.messages.required'))
  if (!values.imap?.username) requireField(ctx, ['imap', 'username'], t('globals.messages.required'))
  if (!values.imap?.password) requireField(ctx, ['imap', 'password'], t('globals.messages.required'))
  if (!values.imap?.mailbox) requireField(ctx, ['imap', 'mailbox'], t('globals.messages.required'))
  if (!values.imap?.read_interval) {
    requireField(ctx, ['imap', 'read_interval'], t('globals.messages.required'))
  } else if (!isGoDuration(values.imap.read_interval)) {
    requireField(ctx, ['imap', 'read_interval'], t('validation.invalidDuration'))
  }
  if (!values.imap?.scan_inbox_since) {
    requireField(ctx, ['imap', 'scan_inbox_since'], t('globals.messages.required'))
  } else if (!isGoDuration(values.imap.scan_inbox_since)) {
    requireField(ctx, ['imap', 'scan_inbox_since'], t('validation.invalidDuration'))
  }
  if (!values.imap?.port || values.imap.port < 1 || values.imap.port > 65535) {
    requireField(ctx, ['imap', 'port'], t('validation.invalidPortValue'))
  }

  if (!values.smtp?.host) requireField(ctx, ['smtp', 'host'], t('globals.messages.required'))
  if (!values.smtp?.username) requireField(ctx, ['smtp', 'username'], t('globals.messages.required'))
  if (!values.smtp?.password) requireField(ctx, ['smtp', 'password'], t('globals.messages.required'))
  if (!values.smtp?.idle_timeout) {
    requireField(ctx, ['smtp', 'idle_timeout'], t('globals.messages.required'))
  } else if (!isGoDuration(values.smtp.idle_timeout)) {
    requireField(ctx, ['smtp', 'idle_timeout'], t('validation.invalidDuration'))
  }
  if (!values.smtp?.pool_wait_timeout) {
    requireField(ctx, ['smtp', 'pool_wait_timeout'], t('globals.messages.required'))
  } else if (!isGoDuration(values.smtp.pool_wait_timeout)) {
    requireField(ctx, ['smtp', 'pool_wait_timeout'], t('validation.invalidDuration'))
  }
  if (!values.smtp?.port || values.smtp.port < 1 || values.smtp.port > 65535) {
    requireField(ctx, ['smtp', 'port'], t('validation.invalidPortValue'))
  }

  if (values.auth_type === AUTH_TYPE_OAUTH2 && !values.oauth?.client_id) {
    requireField(ctx, ['oauth', 'client_id'], t('globals.messages.required'))
  }
})
