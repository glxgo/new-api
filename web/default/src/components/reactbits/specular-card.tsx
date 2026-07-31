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
import {
  forwardRef,
  useRef,
  type CSSProperties,
  type HTMLAttributes,
  type MutableRefObject,
  type PointerEvent,
  type ReactNode,
} from 'react'
import { cn } from '@/lib/utils'

export interface SpecularCardProps extends HTMLAttributes<HTMLDivElement> {
  specularColor?: string
  specularRadius?: number
  specularIntensity?: number
  children?: ReactNode
}

export const SpecularCard = forwardRef<HTMLDivElement, SpecularCardProps>(
  function SpecularCard(
    {
      className,
      specularColor = 'rgba(255,255,255,0.55)',
      specularRadius = 180,
      specularIntensity = 0.9,
      style,
      onPointerMove,
      onPointerLeave,
      children,
      ...rest
    },
    ref
  ) {
    const innerRef = useRef<HTMLDivElement | null>(null)

    const handleMove = (event: PointerEvent<HTMLDivElement>) => {
      const element = innerRef.current
      if (!element || event.pointerType === 'touch') return

      const rect = element.getBoundingClientRect()
      element.style.setProperty('--spec-x', `${event.clientX - rect.left}px`)
      element.style.setProperty('--spec-y', `${event.clientY - rect.top}px`)
      element.dataset.specActive = 'true'
    }

    const handleLeave = () => {
      if (innerRef.current) innerRef.current.dataset.specActive = 'false'
    }

    return (
      <div
        ref={(node) => {
          innerRef.current = node
          if (typeof ref === 'function') ref(node)
          else if (ref) {
            ;(ref as MutableRefObject<HTMLDivElement | null>).current = node
          }
        }}
        data-spec-card=''
        style={
          {
            '--spec-color': specularColor,
            '--spec-radius': `${specularRadius}px`,
            '--spec-intensity': specularIntensity,
            ...style,
          } as CSSProperties
        }
        onPointerMove={(event) => {
          handleMove(event)
          onPointerMove?.(event)
        }}
        onPointerLeave={(event) => {
          handleLeave()
          onPointerLeave?.(event)
        }}
        className={cn('group/spec relative isolate overflow-hidden', className)}
        {...rest}
      >
        {children}
      </div>
    )
  }
)
