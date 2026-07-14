/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'

import type { PricingModel } from '../types'

export function DailyLimitStatus(props: {
  model: PricingModel
  showUnlimited?: boolean
  className?: string
}) {
  const { t } = useTranslation()
  const limit = props.model.daily_request_limit

  if (!limit || limit <= 0) {
    if (!props.showUnlimited) return null
    return (
      <StatusBadge
        label={t('Unlimited')}
        variant='neutral'
        copyable={false}
        size='sm'
        className={props.className}
      />
    )
  }

  const remaining = props.model.daily_request_remaining
  const unavailable = remaining == null
  const exhausted = remaining === 0
  const label = unavailable
    ? t('Availability temporarily unavailable')
    : exhausted
      ? t('Sold out today')
      : t('Today remaining {{remaining}} / {{limit}}', {
          remaining,
          limit,
        })

  return (
    <StatusBadge
      label={label}
      variant={exhausted ? 'danger' : unavailable ? 'neutral' : 'warning'}
      copyable={false}
      size='sm'
      className={props.className}
    />
  )
}
