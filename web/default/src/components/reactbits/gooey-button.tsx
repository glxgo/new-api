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
import { useId, useRef, type ButtonHTMLAttributes, type ReactNode } from 'react'
import { cn } from '@/lib/utils'

export interface GooeyButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  particleCount?: number
  particleDistances?: [number, number]
  particleR?: number
  colors?: number[]
  children?: ReactNode
}

const noise = (n = 1) => n / 2 - Math.random() * n

function getXY(
  distance: number,
  pointIndex: number,
  totalPoints: number
): [number, number] {
  const angle = ((360 + noise(8)) / totalPoints) * pointIndex * (Math.PI / 180)
  return [distance * Math.cos(angle), distance * Math.sin(angle)]
}

export function GooeyButton({
  particleCount = 12,
  particleDistances = [70, 8],
  particleR = 90,
  colors = [1, 2, 3, 1, 2, 3, 1, 4],
  className,
  onClick,
  children,
  ...rest
}: GooeyButtonProps) {
  const fxRef = useRef<HTMLSpanElement | null>(null)
  const filterId = `gb-goo-${useId().replace(/:/g, '')}`

  const makeParticles = (element: HTMLElement) => {
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return

    const d = particleDistances
    const r = particleR
    for (let i = 0; i < particleCount; i++) {
      const t = 850 + noise(450)
      const rotate = noise(r / 10)
      const start = getXY(d[0], particleCount - i, particleCount)
      const end = getXY(d[1] + noise(6), particleCount - i, particleCount)
      const scale = 1 + noise(0.2)
      const color = colors[Math.floor(Math.random() * colors.length)]
      const rotateDeg =
        rotate > 0 ? (rotate + r / 20) * 10 : (rotate - r / 20) * 10

      setTimeout(() => {
        const particle = document.createElement('span')
        const point = document.createElement('span')
        particle.classList.add('gb-particle')
        particle.style.setProperty('--start-x', `${start[0]}px`)
        particle.style.setProperty('--start-y', `${start[1]}px`)
        particle.style.setProperty('--end-x', `${end[0]}px`)
        particle.style.setProperty('--end-y', `${end[1]}px`)
        particle.style.setProperty('--time', `${t}ms`)
        particle.style.setProperty('--scale', `${scale}`)
        particle.style.setProperty('--color', `var(--gb-color-${color}, #fff)`)
        particle.style.setProperty('--rotate', `${rotateDeg}deg`)

        point.classList.add('gb-point')
        particle.appendChild(point)
        element.appendChild(particle)
        requestAnimationFrame(() => {
          element.classList.add('active')
        })
        setTimeout(() => {
          particle.remove()
        }, t)
      }, 30)
    }
  }

  const handleClick = (event: React.MouseEvent<HTMLButtonElement>) => {
    if (fxRef.current) {
      fxRef.current.querySelectorAll('.gb-particle').forEach((node) => {
        node.remove()
      })
      fxRef.current.classList.remove('active')
      void fxRef.current.offsetWidth
      makeParticles(fxRef.current)
    }
    onClick?.(event)
  }

  return (
    <button
      type='button'
      className={cn('gb-button group/gooey', className)}
      onClick={handleClick}
      {...rest}
    >
      <span
        ref={fxRef}
        className='gb-fx'
        style={{ filter: `url(#${filterId})` }}
        aria-hidden='true'
      />
      <span className='gb-content relative z-[3] inline-flex items-center justify-center'>
        {children}
      </span>
      <svg className='gb-svg' aria-hidden='true' focusable='false'>
        <defs>
          <filter id={filterId}>
            <feGaussianBlur in='SourceGraphic' stdDeviation='7' result='blur' />
            <feColorMatrix
              in='blur'
              mode='matrix'
              values='1 0 0 0 0  0 1 0 0 0  0 0 1 0 0  0 0 0 22 -11'
              result='goo'
            />
            <feBlend in='SourceGraphic' in2='goo' />
          </filter>
        </defs>
      </svg>
    </button>
  )
}
