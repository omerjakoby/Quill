import { useState } from 'react'
import './css/App.css'
import Bar from './components/UpperBar'

function App() {
  const [count, setCount] = useState(0)

  return (
    <body>
        <Bar/>
     </body>  
  )
}

export default App
