import { h } from 'vue'
import { RouterLink } from 'vue-router'
import dropdown from './dataTableDropdown.vue'
import { format } from 'date-fns'
import { Badge } from '@shared-ui/components/ui/badge'

export const createColumns = (t) => [
  {
    accessorKey: 'name',
    header: () => h('div', { class: 'text-center' }, t('globals.terms.name')),
    cell: ({ row }) => h('div', { class: 'text-center' }, h(RouterLink, {
      to: { name: 'edit-application', params: { id: row.original.id } },
      class: 'text-primary hover:underline'
    }, () => row.getValue('name')))
  },
  {
    accessorKey: 'slug',
    header: () => h('div', { class: 'text-center' }, t('application.slug')),
    cell: ({ row }) => h('div', { class: 'text-center font-mono' }, row.getValue('slug'))
  },
  {
    accessorKey: 'gateway_app_id',
    header: () => h('div', { class: 'text-center' }, t('application.gatewayAppId')),
    cell: ({ row }) => h('div', { class: 'text-center font-mono text-xs' }, row.getValue('gateway_app_id'))
  },
  {
    accessorKey: 'enabled',
    enableGlobalFilter: false,
    header: () => h('div', { class: 'text-center' }, t('globals.terms.status')),
    cell: ({ row }) => {
      const enabled = row.getValue('enabled')
      return h('div', { class: 'text-center' }, [
        h(Badge, { variant: enabled ? 'default' : 'secondary', class: 'text-xs' }, () =>
          enabled ? t('globals.terms.active') : t('globals.terms.inactive')
        )
      ])
    }
  },
  {
    accessorKey: 'updated_at',
    enableGlobalFilter: false,
    header: () => h('div', { class: 'text-center' }, t('globals.terms.updatedAt')),
    cell: ({ row }) => h('div', { class: 'text-center text-sm' }, format(row.getValue('updated_at'), 'PPpp'))
  },
  {
    id: 'actions',
    enableHiding: false,
    enableSorting: false,
    cell: ({ row }) => h('div', { class: 'relative' }, h(dropdown, { application: row.original }))
  }
]
