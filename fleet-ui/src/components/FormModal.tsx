import { type ReactNode } from 'react'

interface FormModalProps {
  open: boolean
  title: string
  note?: string
  okLabel?: string
  onClose: () => void
  onSubmit: () => void
  children: ReactNode
}

export function FormModal({
  open,
  title,
  note,
  okLabel = 'Save',
  onClose,
  onSubmit,
  children,
}: FormModalProps) {
  if (!open) return null

  return (
    <div className="fmodal open">
      <form
        className="fcard"
        onSubmit={(e) => {
          e.preventDefault()
          onSubmit()
        }}
      >
        <h2>{title}</h2>
        {children}
        <div className="fbtns">
          <button className="fsub" type="submit">
            {okLabel}
          </button>
          <button type="button" className="fcan" onClick={onClose}>
            Cancel
          </button>
        </div>
        {note && <div className="fnote">{note}</div>}
      </form>
    </div>
  )
}

export function FormRow({
  label,
  children,
}: {
  label: string
  children: React.ReactNode
}) {
  return (
    <div className="frow">
      <label className="fl">{label}</label>
      {children}
    </div>
  )
}

export function FormInput({
  value,
  onChange,
  type = 'text',
  placeholder,
  readOnly,
}: {
  value: string
  onChange?: (v: string) => void
  type?: string
  placeholder?: string
  readOnly?: boolean
}) {
  return (
    <input
      className="fi2"
      type={type}
      value={value}
      placeholder={placeholder}
      readOnly={readOnly}
      onChange={onChange ? (e) => onChange(e.target.value) : undefined}
    />
  )
}

export function FormSelect({
  value,
  onChange,
  options,
}: {
  value: string
  onChange: (v: string) => void
  options: string[]
}) {
  return (
    <select className="fi2" value={value} onChange={(e) => onChange(e.target.value)}>
      {options.map((o) => (
        <option key={o} value={o}>
          {o}
        </option>
      ))}
    </select>
  )
}
