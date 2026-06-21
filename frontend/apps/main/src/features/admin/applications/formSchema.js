import * as z from 'zod'

const optionalURL = (t) =>
  z.union([
    z.literal(''),
    z.string().url({ message: t('validation.invalidUrl') })
  ])

export const createFormSchema = (t) =>
  z.object({
    name: z.string({ required_error: t('globals.messages.required') }).min(1, {
      message: t('globals.messages.required')
    }),
    slug: z.string({ required_error: t('globals.messages.required') }).min(1, {
      message: t('globals.messages.required')
    }).regex(/^[a-z0-9]+(?:-[a-z0-9]+)*$/, {
      message: t('application.slugHelp')
    }),
    description: z.string().max(300).optional().default(''),
    logo_url: optionalURL(t).optional().default(''),
    identity_url: optionalURL(t).optional().default(''),
    gateway_app_id: z.string().max(140).optional().default(''),
    gateway_api_key: z.string().optional().default(''),
    enabled: z.boolean().default(true).optional()
  })

