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

Component adapted from react-bits (https://github.com/DavidHDev/react-bits),
licensed under the MIT License.
*/
import {
  Children,
  Fragment,
  useEffect,
  useRef,
  useState,
  useLayoutEffect,
  type ButtonHTMLAttributes,
  type HTMLAttributes,
  type ReactNode,
} from 'react'
import { AnimatePresence, motion, type Variants } from 'motion/react'
import './stepper.css'

interface StepIndicatorRenderProps {
  step: number
  currentStep: number
  onStepClick: (step: number) => void
}

interface StepperProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode
  initialStep?: number
  onStepChange?: (step: number) => void
  onFinalStepCompleted?: () => void | Promise<void>
  // Per-step gate called before advancing to the next step. Return false (or a
  // Promise that resolves to false) to stay on the current step. Used for
  // step-local validation (e.g. require a field before proceeding).
  onBeforeNext?: (step: number) => boolean | Promise<boolean>
  stepCircleContainerClassName?: string
  stepContainerClassName?: string
  contentClassName?: string
  footerClassName?: string
  backButtonProps?: ButtonHTMLAttributes<HTMLButtonElement>
  nextButtonProps?: ButtonHTMLAttributes<HTMLButtonElement>
  backButtonText?: ReactNode
  nextButtonText?: ReactNode
  completeButtonText?: ReactNode
  disableStepIndicators?: boolean
  renderStepIndicator?: (props: StepIndicatorRenderProps) => ReactNode
}

// react-bits Stepper：分步指示器。Create API Key 抽屉里把一次性表单拆成 3 步引导。
export default function Stepper({
  children,
  initialStep = 1,
  onStepChange = () => {},
  onFinalStepCompleted = () => {},
  onBeforeNext,
  stepCircleContainerClassName = '',
  stepContainerClassName = '',
  contentClassName = '',
  footerClassName = '',
  backButtonProps = {},
  nextButtonProps = {},
  backButtonText = 'Back',
  nextButtonText = 'Continue',
  completeButtonText = 'Complete',
  disableStepIndicators = false,
  renderStepIndicator,
  ...rest
}: StepperProps) {
  const [currentStep, setCurrentStep] = useState(initialStep)
  const [direction, setDirection] = useState(0)
  const actionPendingRef = useRef(false)
  const stepsArray = Children.toArray(children)
  const totalSteps = stepsArray.length
  const isCompleted = currentStep > totalSteps
  const isLastStep = currentStep === totalSteps

  const updateStep = (newStep: number) => {
    setCurrentStep(newStep)
    if (newStep <= totalSteps) {
      onStepChange(newStep)
    }
  }
  const handleBack = () => {
    if (currentStep > 1) {
      setDirection(-1)
      updateStep(currentStep - 1)
    }
  }
  const handleNext = async () => {
    if (isLastStep || actionPendingRef.current) return
    actionPendingRef.current = true
    try {
      const ok = onBeforeNext ? await onBeforeNext(currentStep) : true
      if (ok === false) return
      setDirection(1)
      updateStep(currentStep + 1)
    } catch {
      return
    } finally {
      actionPendingRef.current = false
    }
  }
  const handleComplete = async () => {
    if (actionPendingRef.current) return
    actionPendingRef.current = true
    setDirection(1)
    try {
      await onFinalStepCompleted()
      updateStep(totalSteps + 1)
    } finally {
      actionPendingRef.current = false
    }
  }

  return (
    <div className='rb-stepper outer-container' {...rest}>
      <div
        className={`step-circle-container ${stepCircleContainerClassName}`}
        style={{ border: '1px solid var(--border)' }}
      >
        <div className={`step-indicator-row ${stepContainerClassName}`}>
          {stepsArray.map((_, index) => {
            const stepNumber = index + 1
            const isNotLastStep = index < totalSteps - 1
            return (
              <Fragment key={stepNumber}>
                {renderStepIndicator ? (
                  renderStepIndicator({
                    step: stepNumber,
                    currentStep,
                    onStepClick: (clicked) => {
                      setDirection(clicked > currentStep ? 1 : -1)
                      updateStep(clicked)
                    },
                  })
                ) : (
                  <StepIndicator
                    step={stepNumber}
                    disableStepIndicators={disableStepIndicators}
                    currentStep={currentStep}
                    onClickStep={(clicked) => {
                      setDirection(clicked > currentStep ? 1 : -1)
                      updateStep(clicked)
                    }}
                  />
                )}
                {isNotLastStep && (
                  <StepConnector isComplete={currentStep > stepNumber} />
                )}
              </Fragment>
            )
          })}
        </div>
        <StepContentWrapper
          isCompleted={isCompleted}
          currentStep={currentStep}
          direction={direction}
          className={`step-content-default ${contentClassName}`}
        >
          {stepsArray[currentStep - 1]}
        </StepContentWrapper>
        {!isCompleted && (
          <div className={`footer-container ${footerClassName}`}>
            <div
              className={`footer-nav ${currentStep !== 1 ? 'spread' : 'end'}`}
            >
              {currentStep !== 1 && (
                <button
                  onClick={handleBack}
                  type='button'
                  className={`back-button ${currentStep === 1 ? 'inactive' : ''}`}
                  {...backButtonProps}
                >
                  {backButtonText}
                </button>
              )}
              <button
                onClick={isLastStep ? handleComplete : handleNext}
                type='button'
                className='next-button'
                {...nextButtonProps}
              >
                {isLastStep ? completeButtonText : nextButtonText}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function StepContentWrapper({
  isCompleted,
  currentStep,
  direction,
  children,
  className,
}: {
  isCompleted: boolean
  currentStep: number
  direction: number
  children: ReactNode
  className: string
}) {
  const [parentHeight, setParentHeight] = useState(0)
  // The very first height measurement lands while the parent Dialog is still
  // running its open animation, so the measured offsetHeight is unstable and
  // animating 0 -> h on top of the Dialog zoom produces a visible jitter
  // (the "opens big, then shrinks" effect). We therefore skip the height
  // transition for the first measurement (jumping straight to the real
  // height) and only enable the transition for subsequent step changes.
  // A tween (not spring) is used so the height never overshoots.
  const [heightAnimEnabled, setHeightAnimEnabled] = useState(false)
  useEffect(() => {
    if (parentHeight > 0 && !heightAnimEnabled) {
      const id = requestAnimationFrame(() => setHeightAnimEnabled(true))
      return () => cancelAnimationFrame(id)
    }
  }, [parentHeight, heightAnimEnabled])
  return (
    <motion.div
      className={className}
      style={{ position: 'relative', overflow: 'hidden' }}
      animate={{ height: isCompleted ? 0 : parentHeight }}
      transition={
        heightAnimEnabled ? { duration: 0.3, ease: 'easeOut' } : { duration: 0 }
      }
    >
      <AnimatePresence initial={false} mode='sync' custom={direction}>
        {!isCompleted && (
          <SlideTransition
            key={currentStep}
            direction={direction}
            onHeightReady={(h) => setParentHeight(h)}
          >
            {children}
          </SlideTransition>
        )}
      </AnimatePresence>
    </motion.div>
  )
}

function SlideTransition({
  children,
  direction,
  onHeightReady,
}: {
  children: ReactNode
  direction: number
  onHeightReady: (height: number) => void
}) {
  const containerRef = useRef<HTMLDivElement>(null)
  useLayoutEffect(() => {
    if (containerRef.current) onHeightReady(containerRef.current.offsetHeight)
  }, [children, onHeightReady])
  return (
    <motion.div
      ref={containerRef}
      custom={direction}
      variants={stepVariants}
      initial='enter'
      animate='center'
      exit='exit'
      transition={{ duration: 0.4 }}
      style={{ position: 'absolute', left: 0, right: 0, top: 0 }}
    >
      {children}
    </motion.div>
  )
}

const stepVariants: Variants = {
  enter: (dir: number) => ({ x: dir >= 0 ? '-100%' : '100%', opacity: 0 }),
  center: { x: '0%', opacity: 1 },
  exit: (dir: number) => ({ x: dir >= 0 ? '50%' : '-50%', opacity: 0 }),
}

export function Step({ children }: { children: ReactNode }) {
  return <div className='step-default'>{children}</div>
}

const indicatorOuterVariants: Variants = {
  inactive: { scale: 1 },
  active: { scale: 1 },
  complete: { scale: 1 },
}

const indicatorInnerVariants: Variants = {
  inactive: {
    scale: 1,
    backgroundColor: 'var(--border)',
    color: 'var(--muted-foreground)',
  },
  active: {
    scale: 1,
    backgroundColor: 'var(--primary)',
    color: 'var(--primary)',
  },
  complete: {
    scale: 1,
    backgroundColor: 'var(--primary)',
    color: 'var(--info)',
  },
}

function StepIndicator({
  step,
  currentStep,
  onClickStep,
  disableStepIndicators,
}: {
  step: number
  currentStep: number
  onClickStep: (step: number) => void
  disableStepIndicators: boolean
}) {
  const status: 'inactive' | 'active' | 'complete' =
    currentStep === step
      ? 'active'
      : currentStep < step
        ? 'inactive'
        : 'complete'
  const handleClick = () => {
    if (step !== currentStep && !disableStepIndicators) onClickStep(step)
  }
  return (
    <motion.div
      onClick={handleClick}
      className='step-indicator'
      style={
        disableStepIndicators
          ? { pointerEvents: 'none', opacity: 0.5 }
          : undefined
      }
      variants={indicatorOuterVariants}
      animate={status}
      initial={false}
    >
      <motion.div
        variants={indicatorInnerVariants}
        transition={{ duration: 0.3 }}
        className='step-indicator-inner'
      >
        {status === 'complete' ? (
          <CheckIcon className='check-icon' />
        ) : status === 'active' ? (
          <div className='active-dot' />
        ) : (
          <span className='step-number'>{step}</span>
        )}
      </motion.div>
    </motion.div>
  )
}

const lineVariants: Variants = {
  incomplete: { width: 0, backgroundColor: 'transparent' },
  complete: { width: '100%', backgroundColor: 'var(--primary)' },
}

function StepConnector({ isComplete }: { isComplete: boolean }) {
  return (
    <div className='step-connector'>
      <motion.div
        className='step-connector-inner'
        variants={lineVariants}
        initial={false}
        animate={isComplete ? 'complete' : 'incomplete'}
        transition={{ duration: 0.4 }}
      />
    </div>
  )
}

function CheckIcon(props: HTMLAttributes<SVGElement>) {
  return (
    <svg
      {...props}
      fill='none'
      stroke='currentColor'
      strokeWidth={2}
      viewBox='0 0 24 24'
    >
      <motion.path
        initial={{ pathLength: 0 }}
        animate={{ pathLength: 1 }}
        transition={{
          delay: 0.1,
          type: 'tween',
          ease: 'easeOut',
          duration: 0.3,
        }}
        strokeLinecap='round'
        strokeLinejoin='round'
        d='M5 13l4 4L19 7'
      />
    </svg>
  )
}
