import jsQR from 'jsqr'

/** 从本地图片文件识别二维码文本内容。 */
export async function decodeQrFromFile(file: File): Promise<string> {
  if (!file.type.startsWith('image/')) {
    throw new Error('请选择图片文件')
  }
  const bitmap = await createImageBitmap(file)
  try {
    const canvas = document.createElement('canvas')
    canvas.width = bitmap.width
    canvas.height = bitmap.height
    const ctx = canvas.getContext('2d')
    if (!ctx) throw new Error('canvas_unavailable')
    ctx.drawImage(bitmap, 0, 0)
    const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height)
    const result = jsQR(imageData.data, imageData.width, imageData.height, {
      inversionAttempts: 'attemptBoth',
    })
    const text = result?.data?.trim() || ''
    if (!text) throw new Error('未识别到二维码，请换更清晰的图片')
    return text
  } finally {
    bitmap.close()
  }
}
