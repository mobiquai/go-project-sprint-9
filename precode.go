package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Generator генерирует последовательность чисел 1,2,3 и т.д. и
// отправляет их в канал ch. При этом после записи в канал для каждого числа
// вызывается функция fn. Она служит для подсчёта количества и суммы
// сгенерированных чисел.
func Generator(ctx context.Context, ch chan<- int64, fn func(int64)) {
	v := int64(1) // первое число из последовательности

	defer close(ch) // закрываем канал ch по окончании работы функции

	for { // бесконечный цикл - будет выполняться ДО принудительного вызова return
		select {
		case <-ctx.Done(): // выходим из функции, когда контекст завершает работу
			return
		default:
			ch <- v // записываем значение в канал ch
			fn(v)
			v++
		}
	}

}

// Worker читает число из канала in и пишет его в канал out.
func Worker(in <-chan int64, out chan<- int64) {
	defer close(out) // закрываем канал out по окончании работы функции

	for {
		v, ok := <-in // получаем числа из канала
		if !ok {
			break
		}

		out <- v // записываем значение в канал out
		time.Sleep(time.Millisecond)
	}

}

func main() {
	chIn := make(chan int64)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second) // это родительсктй контекст, который завершится через 1 секунду
	defer cancel()

	// для проверки будем считать количество и сумму отправленных чисел
	var inputSum atomic.Int64   // сумма сгенерированных чисел - изменил исходный тип int64 на атомарную для предотварщения потенциального состояния гонки
	var inputCount atomic.Int64 // количество сгенерированных чисел - изменил исходный тип int64 на атомарную для предотварщения потенциального состояния гонки

	// генерируем числа, считая параллельно их количество и сумму
	go Generator(ctx, chIn, func(i int64) {
		//inputSum += i
		//inputCount++
		inputSum.Add(i)
		inputCount.Add(1)
	})

	const NumOut = 5 // количество обрабатывающих горутин и каналов
	// outs — слайс каналов, куда будут записываться числа из chIn
	outs := make([]chan int64, NumOut)
	for i := 0; i < NumOut; i++ {
		// создаём каналы и для каждого из них вызываем горутину Worker
		outs[i] = make(chan int64)
		go Worker(chIn, outs[i])
	}

	// amounts — слайс, в который собирается статистика по горутинам
	amounts := make([]int64, NumOut)
	// chOut — канал, в который будут отправляться числа из горутин `outs[i]`
	chOut := make(chan int64, NumOut)

	var wg sync.WaitGroup

	for i := 0; i < NumOut; i++ {
		wg.Add(1) // увеличиваем счётчик на 1

		go func(in <-chan int64, i int64) {
			defer wg.Done() // уменьшаем счётчик, когда горутина завершает работу

			var sumNumb int64 // счетчик обработанных чисел текущего канала outs[i]
			for n := range in {
				sumNumb += 1
				chOut <- n // отправляем прочитанное число в результирующий канал chOut
			}
			amounts[i] = sumNumb // увеличиваем счыетчик обработанных чисел

		}(outs[i], int64(i))
	}

	go func() {
		// ждём завершения работы всех горутин для outs
		wg.Wait()
		// закрываем результирующий канал
		close(chOut)
	}()

	var count int64 // количество чисел результирующего канала
	var sum int64   // сумма чисел результирующего канала

	for v := range chOut {
		count++
		sum += v
	}

	fmt.Println("Количество чисел", inputCount.Load(), count)
	fmt.Println("Сумма чисел", inputSum.Load(), sum)
	fmt.Println("Разбивка по каналам", amounts)

	// проверка результатов
	if inputSum.Load() != sum {
		log.Fatalf("Ошибка: суммы чисел не равны: %d != %d\n", inputSum.Load(), sum)
	}
	if inputCount.Load() != count {
		log.Fatalf("Ошибка: количество чисел не равно: %d != %d\n", inputCount.Load(), count)
	}
	for _, v := range amounts {
		//inputCount -= v
		inputCount.Add(-v)
	}
	if inputCount.Load() != 0 {
		log.Fatalf("Ошибка: разделение чисел по каналам неверное\n")
	}

}
