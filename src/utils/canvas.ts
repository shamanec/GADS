const addCanvasEventListeners = (
    canvas: HTMLCanvasElement,
    handleMouseDown: (event: MouseEvent) => void,
    handleMouseUp: (event: MouseEvent) => void
) => {
    canvas.addEventListener('mousedown', handleMouseDown)
    canvas.addEventListener('mouseup', handleMouseUp)
}
  
const removeCanvasEventListeners = (
    canvas: HTMLCanvasElement,
    handleMouseDown: (event: MouseEvent) => void,
    handleMouseUp: (event: MouseEvent) => void
) => {
    canvas.removeEventListener('mousedown', handleMouseDown)
    canvas.removeEventListener('mouseup', handleMouseUp)
}

export { addCanvasEventListeners, removeCanvasEventListeners }